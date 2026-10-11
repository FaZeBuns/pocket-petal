package icon

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
)

// The resources a Windows binary carries, by their well-known numbers.
const (
	rtIcon      = 3
	rtGroupIcon = 14
	rtVersion   = 16
)

// Info is what the exe says about itself in Explorer: the version tab, and the
// description a person reads when deciding whether to run it.
type Info struct {
	Name        string // what it is called, as a person sees it
	Slug        string // what the file is called, as a file system sees it
	Description string // one line on what it does
	Version     string // "1.2.3"
	Copyright   string
}

// Table is a resource directory, and the places in it that only the linker can fill
// in: each entry names its own contents by offset from the start of the section, and
// where the section will end up is not known until the binary is linked.
//
// The reason these cannot simply be written as they are is worth stating, because it
// looks like a bug in this file: the resource table of a *linked* binary holds
// addresses, not offsets, and it is the linker that adds the section's address to
// every one of them (see cmd/link/internal/ld/pe.go, addpersrc, which does exactly
// that for the relocations of a .rsrc section). So the offsets go in as they are and
// are listed in Fixups, and the object below turns each of them into a relocation.
// Skip that and the resources are all present in the exe and none of them work: an
// icon that is not drawn, a version that reads as blank.
type Table struct {
	Data   []byte
	Fixups []int // where the four-byte addresses go, in Data
}

// langEnglish is the language every resource here is declared in, and the *order* of the
// three levels is what makes the tree readable: a resource lives at
// /type/name/language, and each level is a directory holding the one below it. It is
// worth the two sentences because both ways of getting it wrong are silent:
//
//	type, name and the bytes   Windows' icon loader accepts it (the language level is
//	                           optional for FindResource) and its *version* reader does
//	                           not — GetFileVersionInfoSize reports the type as absent
//	                           (1813), so the Details tab stays empty
//	type, language, name       nothing at all is found: ExtractIconEx reads no icon
//	                           out of the exe, and no version either
//
// The second is the one this file shipped first, and it cost an afternoon: a resource
// tree is the one format where a wrong directory is not corrupt, it is invisible.
const langEnglish = 0x0409

// Resources builds the resource directory of a Windows binary: the flower at every
// size as the icon, the group that says which sizes belong together, and the version
// stamp. It is the contents of the .rsrc section, laid out the way the format wants
// — identifiers in ascending order, every directory before its contents, and every
// blob on a four-byte boundary.
func Resources(pictures []Picture, info Info) Table {
	icons := &resNode{id: rtIcon}
	for i, pic := range pictures {
		icons.kids = append(icons.kids, leaf(uint32(i+1), pic.Data))
	}
	group := &resNode{id: rtGroupIcon, kids: []*resNode{leaf(1, groupIcon(pictures))}}
	version := &resNode{id: rtVersion, kids: []*resNode{leaf(1, versionInfo(info))}}
	root := &resNode{kids: []*resNode{icons, group, version}}
	return layout(root)
}

// leaf is one resource: a name holding a language holding the bytes.
func leaf(name uint32, data []byte) *resNode {
	return &resNode{id: name, kids: []*resNode{{id: langEnglish, data: data}}}
}

// resNode is one entry of the tree: a directory when it has children, and a leaf
// holding bytes when it does not.
type resNode struct {
	id   uint32
	kids []*resNode
	data []byte
}

// placed is where one node ended up. The tree is walked once to decide that, and
// then written: a resource directory has to know how long it is before anything can
// be pointed at.
type placed struct {
	node    *resNode
	dirAt   int // where the directory's header goes
	entryAt int // for a leaf: where its data entry goes
	dataAt  int // and where the bytes it names go
}

// layout walks the tree, works out where everything goes, and writes it.
func layout(root *resNode) Table {
	var dirs, leaves []*placed
	at := 0
	// the directories come first, in the order they are met
	var walk func(n *resNode)
	walk = func(n *resNode) {
		p := &placed{node: n, dirAt: at}
		at += 16 + 8*len(n.kids) // the header, then one eight-byte entry per child
		dirs = append(dirs, p)
		for _, kid := range n.kids {
			if len(kid.kids) > 0 {
				walk(kid)
				continue
			}
			leaves = append(leaves, &placed{node: kid, entryAt: at})
			at += 16 // a data entry
		}
	}
	walk(root)
	// and then the bytes themselves, each on a four-byte boundary
	for _, leaf := range leaves {
		at = (at + 3) &^ 3
		leaf.dataAt = at
		at += len(leaf.node.data)
	}

	le := binary.LittleEndian
	out := make([]byte, at)
	for _, dir := range dirs {
		le.PutUint16(out[dir.dirAt+14:], uint16(len(dir.node.kids))) // how many there are
		for i, kid := range dir.node.kids {
			e := dir.dirAt + 16 + i*8
			le.PutUint32(out[e:], kid.id)
			if len(kid.kids) > 0 {
				// the high bit says "this is another directory"
				for _, sub := range dirs {
					if sub.node == kid {
						le.PutUint32(out[e+4:], uint32(sub.dirAt)|0x80000000)
						break
					}
				}
				continue
			}
			for _, leaf := range leaves {
				if leaf.node == kid {
					le.PutUint32(out[e+4:], uint32(leaf.entryAt))
					break
				}
			}
		}
	}
	fixups := make([]int, 0, len(leaves))
	for _, leaf := range leaves {
		le.PutUint32(out[leaf.entryAt+0:], uint32(leaf.dataAt))
		le.PutUint32(out[leaf.entryAt+4:], uint32(len(leaf.node.data)))
		// the code page is not interesting for an icon, and zero is how you say so
		copy(out[leaf.dataAt:], leaf.node.data)
		fixups = append(fixups, leaf.entryAt)
	}
	return Table{Data: out, Fixups: fixups}
}

// groupIcon is the RT_GROUP_ICON resource: it names the sizes that make up one icon
// and says which RT_ICON resource holds each of them. The .ico file's directory is
// the same thing with file offsets in place of resource numbers.
func groupIcon(pictures []Picture) []byte {
	le := binary.LittleEndian
	out := make([]byte, 6+14*len(pictures))
	le.PutUint16(out[2:], 1) // 1 is an icon
	le.PutUint16(out[4:], uint16(len(pictures)))
	for i, pic := range pictures {
		e := 6 + i*14
		if pic.Size >= 256 {
			out[e+0], out[e+1] = 0, 0 // 0 means 256
		} else {
			out[e+0], out[e+1] = uint8(pic.Size), uint8(pic.Size)
		}
		le.PutUint16(out[e+4:], 1)  // planes
		le.PutUint16(out[e+6:], 32) // bits a pixel
		le.PutUint32(out[e+8:], uint32(len(pic.Data)))
		le.PutUint16(out[e+12:], uint16(i+1))
	}
	return out
}

// versionInfo builds the VS_VERSIONINFO resource: the version, and the handful of
// strings Explorer shows on the Details tab and reads out to a person who is
// wondering what this file is.
func versionInfo(info Info) []byte {
	major, minor, patch, build := parseVersion(info.Version)

	fixed := make([]byte, 52)
	le := binary.LittleEndian
	le.PutUint32(fixed[0:], 0xFEEF04BD) // the signature every reader looks for
	le.PutUint32(fixed[4:], 0x00010000) // the structure's own version
	le.PutUint32(fixed[8:], word(major, minor))
	le.PutUint32(fixed[12:], word(patch, build))
	le.PutUint32(fixed[16:], word(major, minor)) // the product's version, the same
	le.PutUint32(fixed[20:], word(patch, build))
	le.PutUint32(fixed[24:], 0x3F)    // which of the flags are meaningful
	le.PutUint32(fixed[28:], 0)       // no debug build, no prerelease
	le.PutUint32(fixed[32:], 0x40004) // built for 32-bit Windows: what everything says
	le.PutUint32(fixed[36:], 1)       // an application

	table := vnode{key: "040904B0", text: true} // English, Unicode
	for _, s := range [][2]string{
		{"CompanyName", info.Copyright},
		{"FileDescription", info.Description},
		{"FileVersion", info.Version},
		{"InternalName", info.Slug},
		{"LegalCopyright", info.Copyright},
		{"OriginalFilename", info.Slug + ".exe"},
		{"ProductName", info.Name},
		{"ProductVersion", info.Version},
	} {
		table.kids = append(table.kids, vnode{key: s[0], value: utf16z(s[1]), text: true})
	}
	strings := vnode{key: "StringFileInfo", text: true, kids: []vnode{table}}
	translation := vnode{key: "Translation", value: le32(0x040904B0)}
	vars := vnode{key: "VarFileInfo", kids: []vnode{translation}}

	root := vnode{
		key:   "VS_VERSION_INFO",
		fixed: fixed,
		kids:  []vnode{strings, vars},
	}
	return root.bytes()
}

// vnode is one entry of the version tree. A node is a length, a value length, a type,
// a UTF-16 key, an optional value, and its children — each part on its own four-byte
// boundary, which is the whole of what makes this format fiddly.
type vnode struct {
	key   string
	value []byte // a string in UTF-16, or a raw number
	fixed []byte // the fixed information block: only the root has one
	text  bool   // 1 for text values, 0 for binary
	kids  []vnode
}

func (n vnode) bytes() []byte {
	le := binary.LittleEndian
	// The value length is in bytes for the fixed information and for a number, and in
	// *words* for a string, which is the format's one genuinely surprising rule — and
	// getting it wrong is not a cosmetic error: a reader that does not believe the
	// lengths reports the whole resource as missing.
	length := 0
	value := n.value
	switch {
	case n.fixed != nil:
		length = len(n.fixed)
	case n.value != nil && n.text:
		length = len(n.value) / 2
	case n.value != nil:
		length = len(n.value)
	}
	typ := uint16(0)
	if n.text {
		typ = 1
	}

	head := make([]byte, 6)
	le.PutUint16(head[0:], 0) // the node's length, filled in at the end
	le.PutUint16(head[2:], uint16(length))
	le.PutUint16(head[4:], typ)
	body := append(head, utf16z(n.key)...)

	if n.fixed != nil {
		body = pad(body)
		body = append(body, n.fixed...)
	} else if value != nil {
		body = pad(body)
		body = append(body, value...)
	}
	for _, kid := range n.kids {
		body = pad(body)
		body = append(body, kid.bytes()...)
	}
	body = pad(body)
	le.PutUint16(body[0:], uint16(len(body)))
	return body
}

// pad rounds a node up to the four bytes the format insists on.
func pad(b []byte) []byte {
	for len(b)%4 != 0 {
		b = append(b, 0)
	}
	return b
}

// utf16z is a string the way a version resource holds it: UTF-16, little-endian,
// with the terminator included in the length.
func utf16z(s string) []byte {
	codes := utf16.Encode([]rune(s + "\x00"))
	out := make([]byte, len(codes)*2)
	for i, c := range codes {
		binary.LittleEndian.PutUint16(out[i*2:], c)
	}
	return out
}

func le32(v uint32) []byte {
	out := make([]byte, 4)
	binary.LittleEndian.PutUint32(out, v)
	return out
}

// word packs two halves of a version number into the one number a version resource
// holds them in.
func word(high, low int) uint32 { return uint32(high)<<16 | uint32(low)&0xffff }

// parseVersion reads "1.2.3.4" — or as much of it as there is.
func parseVersion(v string) (major, minor, patch, build int) {
	parts := strings.FieldsFunc(v, func(r rune) bool { return r == '.' || r == '-' })
	nums := make([]int, 4)
	for i, p := range parts {
		if i >= 4 {
			break
		}
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			break
		}
		nums[i] = n
	}
	return nums[0], nums[1], nums[2], nums[3]
}

// Syso wraps a resource table in the object file a Go build links in.
//
// There is no resource compiler here and there does not need to be: a .syso is a COFF
// object with a single section in it, a section symbol so that the relocations below
// have something to point at, and one relocation for each address the linker has to
// fill in. The Go linker copies a section named .rsrc into the resource directory of
// the exe it is making and rewrites those four-byte fields to the section's address
// as it goes. Which means the icon and the version stamp are part of the code like
// everything else: `go build` and they are there.
func Syso(table Table, arch string) ([]byte, error) {
	var machine uint16
	var reloc uint16
	switch arch {
	case "amd64":
		machine, reloc = 0x8664, relAMD64Addr32
	case "arm64":
		machine, reloc = 0xAA64, relARM64Addr32
	case "386":
		machine, reloc = 0x014C, relI386Dir32
	default:
		return nil, fmt.Errorf("icon: no COFF machine for %q", arch)
	}

	const headerSize = 20 + 40
	data := table.Data
	// the order every COFF object is written in: the section's bytes, its
	// relocations, the symbol table, and then the string table, which is found by
	// counting the symbols rather than by being pointed at
	relocAt := align(headerSize+len(data), 4)
	symAt := align(relocAt+relocationSize*len(table.Fixups), 4)
	end := symAt + symbolSize + 4 // the last four bytes are the empty string table

	out := make([]byte, end)
	le := binary.LittleEndian
	le.PutUint16(out[0:], machine)
	le.PutUint16(out[2:], 1) // one section
	le.PutUint32(out[8:], uint32(symAt))
	le.PutUint32(out[12:], 1) // one symbol: the section itself

	copy(out[20:28], ".rsrc\x00\x00\x00")
	le.PutUint32(out[28:], uint32(len(data))) // virtual size
	le.PutUint32(out[36:], uint32(len(data))) // size of the raw data
	le.PutUint32(out[40:], headerSize)        // and where it starts
	if len(table.Fixups) > 0 {
		le.PutUint32(out[44:], uint32(relocAt)) // then the relocations
		le.PutUint16(out[52:], uint16(len(table.Fixups)))
	}
	le.PutUint32(out[56:], 0x40000040) // initialised data, readable
	copy(out[headerSize:], data)

	// the relocations: "the four bytes at this offset are an address in this section"
	for i, at := range table.Fixups {
		e := relocAt + i*relocationSize
		le.PutUint32(out[e:], uint32(at))
		le.PutUint32(out[e+4:], 0) // symbol 0: the section symbol just below
		le.PutUint16(out[e+8:], reloc)
	}

	// the section symbol: a name, a value, which section it is in, and that it is
	// static. Its value is zero, so the addend of every relocation stays what is in
	// the data — the offset of the thing being pointed at.
	copy(out[symAt:], ".rsrc\x00\x00\x00")
	le.PutUint16(out[symAt+12:], 1) // section 1
	out[symAt+16] = 3               // IMAGE_SYM_CLASS_STATIC
	// and the string table, which is empty: the four bytes are its size
	return out, nil
}

// Relocation types, per architecture, out of the PE specification.
const (
	relI386Dir32   = 0x0006
	relAMD64Addr32 = 0x0002
	relARM64Addr32 = 0x0001
)

const (
	relocationSize = 10
	symbolSize     = 18
)

func align(n, to int) int { return (n + to - 1) &^ (to - 1) }
