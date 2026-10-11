package icon

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/FaZeBuns/pocket-petal/internal/flower"
)

// The icon is written by hand — an .ico byte by byte, and a resource directory laid out
// and then finished by the linker — which is exactly why it is worth reading back. The
// two failure modes that matter are both quiet ones: a malformed .ico is a blank space
// in Explorer, and a malformed resource tree is *invisible*, with the exe's face simply
// not drawn and nothing anywhere saying so.
//
// So these tests walk both formats from the top, the way Windows does, including the one
// thing that cannot be seen from the writing side: the order of the three levels a
// resource lives at. Type, then name, then language — and both ways of getting that
// wrong were measured against Windows rather than guessed at (see the comment on
// langEnglish in rsrc.go).

// picturesForTest is a small icon: one bitmap size, and the 256 that is stored as a PNG.
func picturesForTest(t *testing.T) []Picture {
	t.Helper()
	seed := 0
	rig := flower.CreateRig(flower.RigOptions{Seed: &seed, Palette: "blush"})
	pics := Pictures(rig, []int{16, 256}, "bloom")
	if len(pics) != 2 {
		t.Fatalf("the rig drew %d sizes of the icon and 2 were asked for", len(pics))
	}
	return pics
}

func infoForTest() Info {
	return Info{
		Name:        "Pocket Petal",
		Slug:        "pocket-petal",
		Description: "A flower you can pick up and put on someone's chat",
		Version:     "1.2.3",
		Copyright:   "MIT licensed",
	}
}

// The .ico: a header, one directory entry per size, and the pictures themselves — each
// at the offset and length its entry claims, and each actually *being* that size.
func TestTheIcoCarriesEverySizeItSaysItDoes(t *testing.T) {
	pics := picturesForTest(t)
	file := File(pics)
	le := binary.LittleEndian
	if len(file) < 6+16*len(pics) {
		t.Fatalf("the .ico is %d bytes, which does not hold its own directory", len(file))
	}
	if got := le.Uint16(file[2:]); got != 1 {
		t.Errorf("the file is a %d, and 1 is an icon (2 is a cursor)", got)
	}
	if got := le.Uint16(file[4:]); int(got) != len(pics) {
		t.Fatalf("the header says %d pictures and %d were packed", got, len(pics))
	}
	for i, pic := range pics {
		e := 6 + i*16
		w, h := int(file[e+0]), int(file[e+1])
		if w == 0 || h == 0 {
			w, h = 256, 256 // a 256 is stored as a zero byte: the field is eight bits
		}
		if w != pic.Size || h != pic.Size {
			t.Errorf("entry %d says %dx%d and is the %d-pixel flower", i, w, h, pic.Size)
		}
		size := int(le.Uint32(file[e+8:]))
		at := int(le.Uint32(file[e+12:]))
		if size != len(pic.Data) {
			t.Errorf("entry %d says %d bytes and its picture is %d", i, size, len(pic.Data))
		}
		if at < 6+16*len(pics) || at+size > len(file) {
			t.Fatalf("entry %d points at %d..%d, outside a %d-byte file", i, at, at+size, len(file))
		}
		// and the picture is the size the entry advertised: a bitmap whose height
		// counts the mask row as well, or a PNG
		body := file[at : at+size]
		if pic.Size >= 256 {
			if !bytes.HasPrefix(body, []byte("\x89PNG\r\n\x1a\n")) {
				t.Errorf("the %d-pixel picture is not a PNG: % x", pic.Size, body[:8])
			}
			continue
		}
		if got := int(le.Uint32(body[0:])); got != 40 {
			t.Errorf("the %d-pixel bitmap header is %d bytes", pic.Size, got)
		}
		if got := int(le.Uint32(body[4:])); got != pic.Size {
			t.Errorf("the bitmap is %d pixels wide and is the %d-pixel flower", got, pic.Size)
		}
		if got := int(le.Uint32(body[8:])); got != pic.Size*2 {
			t.Errorf("the bitmap is %d pixels tall, and an icon's height is the colours plus the mask", got)
		}
	}
}

// res is one entry of a resource directory, read back the way Windows reads it.
type res struct {
	id  uint32
	at  int  // where it points, counted from the start of the section
	dir bool // a directory (more entries) rather than a data entry (the bytes)
}

// entries reads one resource directory.
func entries(t *testing.T, data []byte, at int) []res {
	t.Helper()
	if at+16 > len(data) {
		t.Fatalf("a directory at %d is outside the section (%d bytes)", at, len(data))
	}
	n := int(binary.LittleEndian.Uint16(data[at+14:]))
	out := make([]res, 0, n)
	for i := 0; i < n; i++ {
		e := at + 16 + i*8
		if e+8 > len(data) {
			t.Fatalf("entry %d of the directory at %d is outside the section", i, at)
		}
		raw := binary.LittleEndian.Uint32(data[e+4:])
		out = append(out, res{
			id:  binary.LittleEndian.Uint32(data[e:]),
			at:  int(raw & 0x7fffffff),
			dir: raw&0x80000000 != 0,
		})
	}
	return out
}

// bytesAt reads one data entry: the bytes it names.
func bytesAt(t *testing.T, data []byte, at int) []byte {
	t.Helper()
	if at+16 > len(data) {
		t.Fatalf("a data entry at %d is outside the section", at)
	}
	le := binary.LittleEndian
	off, size := int(le.Uint32(data[at:])), int(le.Uint32(data[at+4:]))
	if off < 0 || size < 0 || off+size > len(data) {
		t.Fatalf("a data entry holds %d..%d, outside a %d-byte section", off, off+size, len(data))
	}
	return data[off : off+size]
}

// tree is a resource directory read back: what each type holds, and where every data
// entry in it lives.
type tree struct {
	byType map[uint32][][]byte
	data   []int // the addresses of the data entries, which is the list the linker fixes
}

// walk reads the whole tree as /type/name/language/bytes, which is both a test in itself
// and how the tests below ask for the icon, the group and the version block.
func walk(t *testing.T, table Table) tree {
	t.Helper()
	out := tree{byType: map[uint32][][]byte{}}
	for _, ty := range entries(t, table.Data, 0) {
		if !ty.dir {
			t.Fatalf("type %d points straight at bytes: there is no name level under it", ty.id)
		}
		names := entries(t, table.Data, ty.at)
		if len(names) == 0 {
			t.Fatalf("type %d has nothing under it", ty.id)
		}
		for _, name := range names {
			if !name.dir {
				t.Fatalf("type %d, name %d holds its bytes where the language level belongs: "+
					"Windows' version reader reads no version out of a tree like this, and "+
					"ExtractIconEx reads no icon out of one whose levels are swapped",
					ty.id, name.id)
			}
			langs := entries(t, table.Data, name.at)
			if len(langs) != 1 {
				t.Fatalf("type %d, name %d is declared in %d languages and the app writes one",
					ty.id, name.id, len(langs))
			}
			if langs[0].dir {
				t.Fatalf("type %d, name %d, language %d is a directory, and a resource ends in bytes",
					ty.id, name.id, langs[0].id)
			}
			if langs[0].id != langEnglish {
				t.Errorf("type %d, name %d is declared in language 0x%x, and 0x%x is what the version block claims",
					ty.id, name.id, langs[0].id, langEnglish)
			}
			out.byType[ty.id] = append(out.byType[ty.id], bytesAt(t, table.Data, langs[0].at))
			out.data = append(out.data, langs[0].at)
		}
	}
	return out
}

// The tree the exe carries: the flower at every size, the group that names them, and the
// version.
func TestTheResourceTreeHoldsTheIconTheGroupAndTheVersion(t *testing.T) {
	pics := picturesForTest(t)
	found := walk(t, Resources(pics, infoForTest()))

	if len(found.byType) != 3 {
		t.Fatalf("the tree holds %d resource types and the app writes 3", len(found.byType))
	}
	icons := found.byType[rtIcon]
	if len(icons) != len(pics) {
		t.Fatalf("%d icons and %d pictures", len(icons), len(pics))
	}
	for i, pic := range pics {
		if !bytes.Equal(icons[i], pic.Data) {
			t.Errorf("icon %d of the exe is not the %d-pixel picture that was packed", i+1, pic.Size)
		}
	}
	group := found.byType[rtGroupIcon]
	if len(group) != 1 {
		t.Fatalf("there are %d group icon resources", len(group))
	}
	if names := groupIconSizes(t, group[0]); !sameSizes(names, pics) {
		t.Errorf("the group icon names %v and the icon carries %v", names, sizes(pics))
	}
	if len(found.byType[rtVersion]) != 1 {
		t.Fatalf("there are %d version resources", len(found.byType[rtVersion]))
	}
}

// The version block says how long it is, and Windows' own reader checks that first: a
// block whose wLength disagrees with the resource reports the whole resource as absent
// (GetFileVersionInfoSize returns 0 with ERROR_RESOURCE_TYPE_NOT_FOUND), which is how the
// Details tab stays empty for an exe that has a perfectly good version inside it.
func TestTheVersionBlockSaysHowLongItIs(t *testing.T) {
	blob := walk(t, Resources(picturesForTest(t), infoForTest())).byType[rtVersion][0]
	le := binary.LittleEndian
	if len(blob) < 92 {
		t.Fatalf("the version block is %d bytes: a header, the fixed information and a string table do not fit", len(blob))
	}
	if got := int(le.Uint16(blob)); got != len(blob) {
		t.Errorf("the block says it is %d bytes and the resource holds %d", got, len(blob))
	}
	if got := int(le.Uint16(blob[2:])); got != 52 {
		t.Errorf("the fixed information is declared %d bytes long and VS_FIXEDFILEINFO is 52", got)
	}
	// the signature and the structure's own version, which are the two numbers every
	// reader of a version resource looks at before it believes anything else
	if got := le.Uint32(blob[40:]); got != 0xFEEF04BD {
		t.Errorf("the fixed information starts with 0x%08X and not the signature 0xFEEF04BD", got)
	}
	if got := le.Uint32(blob[44:]); got != 0x00010000 {
		t.Errorf("the structure's version is 0x%08X and not 1.0", got)
	}
	// the handful of strings a person reads on the Details tab
	for _, key := range []string{"VS_VERSION_INFO", "StringFileInfo", "040904B0", "ProductName", "Pocket Petal", "1.2.3"} {
		if !bytes.Contains(blob, utf16z(key)) {
			t.Errorf("the version block does not mention %q", key)
		}
	}
}

// The object the linker picks up: one .rsrc section, one relocation for every address
// only the linker knows, and — just as important — *no* relocation for the offsets that
// are counted from the start of the section. That is what a linker-made binary holds too
// (checked against notepad.exe, whose directory entries are offsets like 0x38), and a
// relocator that "helpfully" fixed those up would move every directory in the tree by the
// section's address.
func TestTheSysoRelocatesEveryAddressAndOnlyThose(t *testing.T) {
	table := Resources(picturesForTest(t), infoForTest())
	obj, err := Syso(table, "amd64")
	if err != nil {
		t.Fatalf("Syso: %v", err)
	}
	le := binary.LittleEndian
	const headerSize = 20 + 40
	if got := le.Uint16(obj); got != 0x8664 {
		t.Errorf("the object is for machine 0x%x and amd64 is 0x8664", got)
	}
	if got := le.Uint16(obj[2:]); got != 1 {
		t.Fatalf("the object has %d sections and the resources are one", got)
	}
	if got := strings.TrimRight(string(obj[20:28]), "\x00"); got != ".rsrc" {
		t.Errorf("the section is called %q", got)
	}
	if got := int(le.Uint32(obj[28:])); got != len(table.Data) {
		t.Errorf("the section's size is %d and its data is %d", got, len(table.Data))
	}
	if !bytes.Equal(obj[headerSize:headerSize+len(table.Data)], table.Data) {
		t.Fatal("the object's section is not the table that was built")
	}

	// every data entry in the tree gets a relocation — the check that would have caught
	// the icon coming out of the linker as nothing at all — and nothing else does
	want := walk(t, table).data
	if got := int(le.Uint16(obj[52:])); got != len(want) {
		t.Fatalf("the object carries %d relocations and the tree holds %d data entries", got, len(want))
	}
	if len(table.Fixups) != len(want) {
		t.Fatalf("the table lists %d addresses and its tree holds %d", len(table.Fixups), len(want))
	}
	inTree := map[int]bool{}
	for _, at := range want {
		inTree[at] = true
	}
	relocAt := int(le.Uint32(obj[44:]))
	for i, at := range table.Fixups {
		if !inTree[at] {
			t.Errorf("the table asks the linker to fix %d, and no data entry lives there", at)
			continue
		}
		e := relocAt + i*relocationSize
		if got := int(le.Uint32(obj[e:])); got != at {
			t.Errorf("relocation %d fixes the address at %d and the table wants %d", i, got, at)
		}
		if got := le.Uint32(obj[e+4:]); got != 0 {
			t.Errorf("relocation %d names symbol %d and there is only the section symbol", i, got)
		}
		if got := le.Uint16(obj[e+8:]); got != relAMD64Addr32 {
			t.Errorf("relocation %d is type 0x%x and an amd64 four-byte address is 0x%x", i, got, relAMD64Addr32)
		}
	}
}

// and the offsets, which nobody fixes up: they have to be inside the section, and they
// have to be the section's own, since that is how Windows reads the tree (the walk above
// fails loudly if any of them points outside the bytes that were built).
func TestTheLinkerFillsInAddressesAndNotOffsets(t *testing.T) {
	table := Resources(picturesForTest(t), infoForTest())
	walk(t, table)
	for _, at := range table.Fixups {
		if at%4 != 0 || at+4 > len(table.Data) {
			t.Errorf("the address at %d is not a four-byte field inside the section", at)
		}
		if got := int(binary.LittleEndian.Uint32(table.Data[at:])); got >= len(table.Data) {
			t.Errorf("the field at %d holds %d, which is already past the section", at, got)
		}
	}
}

// groupIconSizes reads the sizes named by a group icon resource.
func groupIconSizes(t *testing.T, blob []byte) []int {
	t.Helper()
	if len(blob) < 6 {
		t.Fatalf("the group icon resource is %d bytes", len(blob))
	}
	n := int(binary.LittleEndian.Uint16(blob[4:]))
	out := make([]int, 0, n)
	for i := 0; i < n; i++ {
		e := 6 + i*14
		if e+14 > len(blob) {
			t.Fatalf("entry %d of the group icon is outside the resource", i)
		}
		size := int(blob[e+0])
		if size == 0 {
			size = 256
		}
		out = append(out, size)
	}
	return out
}

func sizes(pics []Picture) []int {
	out := make([]int, 0, len(pics))
	for _, pic := range pics {
		out = append(out, pic.Size)
	}
	return out
}

func sameSizes(a []int, pics []Picture) bool {
	b := sizes(pics)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
