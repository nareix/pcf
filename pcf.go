package pcf

import (
	"io"
	"fmt"
	"log"
	"os"
	"reflect"
	"unsafe"

	_ "image"
)

const (
	PCF_PROPERTIES       = (1 << 0)
	PCF_ACCELERATORS     = (1 << 1)
	PCF_METRICS          = (1 << 2)
	PCF_BITMAPS          = (1 << 3)
	PCF_INK_METRICS      = (1 << 4)
	PCF_BDF_ENCODINGS    = (1 << 5)
	PCF_SWIDTHS          = (1 << 6)
	PCF_GLYPH_NAMES      = (1 << 7)
	PCF_BDF_ACCELERATORS = (1 << 8)

	PCF_DEFAULT_FORMAT	= 0x00000000
	PCF_INKBOUNDS		= 0x00000200
	PCF_ACCEL_W_INKBOUNDS =	0x00000100
	PCF_COMPRESSED_METRICS	= 0x00000100
)

type tocEntry struct {
	typ    uint32
	format uint32
	size   uint32
	offset uint32
}

type fileHeader struct {
	header     [4]byte
	tableCount int
}

type metricEntry struct {
	leftSidedBearing int
	rightSidedBearing int
	charWidth int
	charAscent int
	charDescent int
	charAttr int
}

type metricTable struct {
	table *tocEntry
	format int32
	count int
}

func (t *metricTable) read(f io.ReadSeeker) (err error) {
	if _, err = f.Seek(int64(t.table.offset), 0); err != nil {
		return
	}
	if err = bread(f, &t.format); err != nil {
		return
	}
	if (t.table.format & PCF_COMPRESSED_METRICS) != 0 {
		var count int16
		if err = breadSwap(f, &count); err != nil {
			return
		}
		t.count = int(count)
	} else {
		var count int32
		if err = breadSwap(f, &count); err != nil {
			return
		}
		t.count = int(count)
	}
	return
}

func (t *metricTable) readMeticEntry(r io.ReadSeeker, i int, entry *metricEntry) (err error) {
	if i > t.count {
		err = fmt.Errorf("readMeticEntry: out of range (%d of %d)", i, t.count)
		return
	}
	if (t.table.format & PCF_COMPRESSED_METRICS) != 0 {
		if _, err = r.Seek(int64(t.table.offset) + 6 + int64(i)*5, 0); err != nil {
			return
		}
		var b [5]byte
		if _, err = r.Read(b[:]); err != nil {
			return
		}
		entry.leftSidedBearing = int(b[0])
		entry.leftSidedBearing -= 0x80
		entry.rightSidedBearing = int(b[1])
		entry.rightSidedBearing -= 0x80
		entry.charWidth = int(b[2])
		entry.charWidth -= 0x80
		entry.charAscent = int(b[3])
		entry.charAscent -= 0x80
		entry.charDescent = int(b[4])
		entry.charDescent -= 0x80
	} else {
		if _, err = r.Seek(int64(t.table.offset) + 8 + int64(i)*12, 0); err != nil {
			return
		}
		var b [6]int16
		if err = bread(r, b); err != nil {
			return
		}
		entry.leftSidedBearing = int(b[0])
		entry.rightSidedBearing = int(b[1])
		entry.charWidth = int(b[2])
		entry.charAscent = int(b[3])
		entry.charDescent = int(b[4])
		entry.charAttr = int(b[5])
	}
	return
}

type bitmapTable struct {
	table *tocEntry
	format int32
	count int32
	offsets []int32
	bitmapSizes [4]int32
}

func (t *bitmapTable) read(r io.ReadSeeker) (err error) {
	if _, err = r.Seek(int64(t.table.offset), 0); err != nil {
		return
	}
	if err = bread(r, &t.format); err != nil {
		return
	}
	if err = breadSwap(r, &t.count); err != nil {
		return
	}
	t.offsets = make([]int32, t.count)
	if err = breadSwap(r, t.offsets); err != nil {
		return
	}
	if err = breadSwap(r, t.bitmapSizes[:]); err != nil {
		return
	}

	if Debug {
		log.Println("bitmap sizes", t.bitmapSizes, t.format&3)
	}
	return
}

func (t *bitmapTable) readData(r io.ReadSeeker, i int) (b []byte, err error) {
	if i+1 > int(t.count) {
		err = fmt.Errorf("bitmapReadData: out of range (%d of %d)", i, t.count)
		return
	}
	off := int64(t.table.offset) + int64(8 + 4*len(t.offsets) + 16)
	off += int64(t.offsets[i])
	size := t.offsets[i+1] - t.offsets[i]
	if size < 0 {
		err = fmt.Errorf("bitmapReadData: invalid offsets")
		return
	}
	if _, err = r.Seek(off, 0); err != nil {
		return
	}
	b = make([]byte, size)
	_, err = r.Read(b)
	return
}

type encodingTable struct {
	table *tocEntry
	format int32
	minCharOrByte2 int16
	maxCharOrByte2 int16
	minByte1 int16
	maxByte1 int16
	defChar int16
	index []int16
}

func (t *encodingTable) read(r io.ReadSeeker) (err error) {
	if _, err = r.Seek(int64(t.table.offset), 0); err != nil {
		return
	}
	if err = bread(r, &t.format); err != nil {
		return
	}
	if err = breadSwap(r, &t.minCharOrByte2); err != nil {
		return
	}
	if err = breadSwap(r, &t.maxCharOrByte2); err != nil {
		return
	}
	if err = breadSwap(r, &t.minByte1); err != nil {
		return
	}
	if err = breadSwap(r, &t.maxByte1); err != nil {
		return
	}
	if err = breadSwap(r, &t.defChar); err != nil {
		return
	}
	size := int(t.maxCharOrByte2-t.minCharOrByte2+1) * int(t.maxByte1-t.minByte1+1)
	t.index = make([]int16, size)
	if err = breadSwap(r, t.index); err != nil {
		return
	}

	if Debug {
		log.Println("encoding table:", t.minCharOrByte2, t.maxCharOrByte2, t.minByte1, t.maxByte1, size)
	}

	return
}

func (t *encodingTable) lookup(i int) (r int, err error) {
	b1, b2 := i&0xff, i>>8
	off := 0
	if b2 == 0 {
		off = b1-int(t.minCharOrByte2)
	} else {
		off = (b2-int(t.minByte1))*int(t.maxCharOrByte2-t.minCharOrByte2+1) +
				(b1-int(t.minCharOrByte2))
	}

	if Debug {
		log.Println("lookup", i, off, b1, b2)
	}

	r = int(t.index[off])
	return
}

func _bread(r io.Reader, v interface{}, swap bool) error {
	rv := reflect.ValueOf(v)
	slice := []byte{}
	rslice := (*reflect.SliceHeader)(unsafe.Pointer(&slice))

	nelem := 0
	switch rv.Type().Kind() {
	case reflect.Ptr:
		nelem = 1
	case reflect.Slice:
		nelem = rv.Len()
	default:
		return fmt.Errorf("_bread: unsupported type")
	}
	size := int(rv.Type().Elem().Size())
	rslice.Data = rv.Pointer()
	rslice.Len = size * nelem
	rslice.Cap = rslice.Len

	n, err := r.Read(slice)
	if n != rslice.Len {
		return err
	}

	if swap {
		for i := 0; i < nelem; i++ {
			start := i*size
			for j := 0; j < size/2; j++ {
				slice[start+j], slice[start+size-1-j] = slice[start+size-1-j], slice[start+j]
			}
		}
	}

	return nil
}

func breadSwap(r io.Reader, v interface{}) error {
	return _bread(r, v, true)
}

func bread(r io.Reader, v interface{}) error {
	return _bread(r, v, false)
}

type PCFFile struct {
	encoding *encodingTable
	bitmap *bitmapTable
	metric *metricTable
	f *os.File
}

var Debug bool

func Open(file string) (pf *PCFFile, err error) {
	var f *os.File
	f, err = os.Open(file)
	if err != nil {
		return
	}

	var fh fileHeader
	if err = bread(f, &fh); err != nil {
		return
	}

	pf = &PCFFile{f: f}

	var tocMetrics, tocBitmaps *tocEntry
	var tocEncoding *tocEntry

	if Debug {
		log.Println("tableCount:", fh.tableCount)
	}
	for i := 0; i < fh.tableCount; i++ {
		toc := &tocEntry{}
		if err = bread(f, toc); err != nil {
			return
		}
		switch toc.typ {
		case PCF_METRICS:
			tocMetrics = toc
		case PCF_BITMAPS:
			tocBitmaps = toc
		case PCF_BDF_ENCODINGS:
			tocEncoding = toc
		}
	}

	if tocMetrics == nil || tocBitmaps == nil {
		err = fmt.Errorf("metrics or bitmap toc not found")
		log.Println(err)
		return
	}
	if tocEncoding == nil {
		err = fmt.Errorf("encoding toc not found")
		log.Println(err)
		return
	}

	pf.metric = &metricTable{table: tocMetrics}
	if err = pf.metric.read(f); err != nil {
		return
	}

	if Debug {
		log.Println("total metric", pf.metric.count)
	}

	pf.bitmap = &bitmapTable{table: tocBitmaps}
	if err = pf.bitmap.read(f); err != nil {
		return
	}

	if Debug {
		log.Println("total bitmap glyphs", pf.bitmap.count)
	}

	pf.encoding = &encodingTable{table: tocEncoding}
	if err = pf.encoding.read(f); err != nil {
		return
	}

	return
}

func (pf *PCFFile) Lookup(r rune) (b []byte, width int, err error) {
	var i int
	if i, err = pf.encoding.lookup(int(r)); err != nil {
		return
	}
	if b, err = pf.bitmap.readData(pf.f, i); err != nil {
		return
	}
	width = 4
	return
}

func (pf *PCFFile) DumpAscii(fname string, r rune) {
	f, err := os.Create(fname)
	if err != nil {
		return
	}

	var b []byte
	var width int

	b, width, err = pf.Lookup(r)
	if Debug {
		log.Println("len", len(b))
	}

	for i := 0; i < len(b); i += width {
		for j := 0; j < width; j++ {
			bits := b[i+j]
			for k := 7; k >= 0; k-- {
				if bits & (1<<byte(k)) != 0 {
					f.Write([]byte{'@'})
				} else {
					f.Write([]byte{'.'})
				}
			}
		}
		f.Write([]byte{'\n'})
	}
	f.Close()
}


