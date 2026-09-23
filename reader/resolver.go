// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"bytes"
	"compress/zlib"
	"fmt"

	"github.com/carlos7ags/folio/core"
)

// resolver fetches and caches PDF objects by their object number.
type resolver struct {
	data       []byte
	xref       *xrefTable
	cache      map[int]core.PdfObject
	maxCache   int            // max cached objects (0 = unlimited)
	order      []int          // insertion order for LRU eviction
	mem        *memoryTracker // memory safety limits
	resolving  map[int]bool   // tracks objects currently being resolved (cycle detection)
	strictness Strictness     // controls error handling behavior
}

// newResolver creates a resolver that fetches objects from data using the
// given xref table, enforcing the specified memory limits and strictness.
func newResolver(data []byte, xref *xrefTable, mem *memoryTracker, strictness Strictness) *resolver {
	return &resolver{
		data:       data,
		xref:       xref,
		cache:      make(map[int]core.PdfObject),
		maxCache:   10000, // default: cache up to 10K objects
		mem:        mem,
		resolving:  make(map[int]bool),
		strictness: strictness,
	}
}

// SetMaxCache sets the maximum number of cached objects.
// When exceeded, the oldest entries are evicted. 0 = unlimited.
func (r *resolver) SetMaxCache(n int) {
	r.maxCache = n
}

// Release removes a cached object, freeing memory.
func (r *resolver) Release(objNum int) {
	delete(r.cache, objNum)
}

// cacheObject stores an object and evicts old entries if needed.
func (r *resolver) cacheObject(objNum int, obj core.PdfObject) {
	r.cache[objNum] = obj
	r.order = append(r.order, objNum)

	if r.maxCache > 0 && len(r.cache) > r.maxCache {
		// Evict oldest 10%.
		evictCount := r.maxCache / 10
		if evictCount < 1 {
			evictCount = 1
		}
		evicted := 0
		newOrder := r.order[:0]
		for _, num := range r.order {
			if evicted < evictCount {
				delete(r.cache, num)
				evicted++
			} else {
				newOrder = append(newOrder, num)
			}
		}
		r.order = newOrder
	}
}

// Resolve returns the PDF object for the given object number.
// Follows indirect references recursively.
// Results are cached. Circular references are detected and return an error.
func (r *resolver) Resolve(objNum int) (core.PdfObject, error) {
	if obj, ok := r.cache[objNum]; ok {
		return obj, nil
	}

	// Detect circular references.
	if r.resolving[objNum] {
		return nil, fmt.Errorf("reader: circular reference detected for object %d", objNum)
	}
	r.resolving[objNum] = true
	defer delete(r.resolving, objNum)

	entry, ok := r.xref.entries[objNum]
	if !ok {
		return core.NewPdfNull(), nil // unknown object → null
	}
	if !entry.inUse {
		return core.NewPdfNull(), nil // free object → null
	}

	// Check if this is a compressed object (type 2 xref entry).
	// For type 2: offset = object stream number, generation = index within stream.
	if entry.compressed {
		return r.resolveCompressed(objNum, int(entry.offset), entry.generation)
	}

	// Validate offset before seeking.
	if entry.offset < 0 || int(entry.offset) >= len(r.data) {
		return nil, fmt.Errorf("reader: object %d has invalid offset %d (file size %d)", objNum, entry.offset, len(r.data))
	}

	// Seek to the object offset and parse.
	tok := NewTokenizer(r.data)
	tok.SetPos(int(entry.offset))
	parser := NewParser(tok)

	parsedObjNum, _, obj, err := parser.ParseIndirectObject()
	if err != nil {
		return nil, fmt.Errorf("reader: resolve object %d: %w", objNum, err)
	}
	if parsedObjNum != objNum {
		return nil, fmt.Errorf("reader: expected object %d at offset %d, got %d", objNum, entry.offset, parsedObjNum)
	}

	// If it's a stream, read the actual stream data from the file.
	if stream, ok := obj.(*core.PdfStream); ok {
		obj, err = r.resolveStream(stream, entry.offset)
		if err != nil {
			return nil, err
		}
	}

	r.cacheObject(objNum, obj)
	return obj, nil
}

// resolveCompressed extracts an object from an object stream.
// objStreamNum is the object number of the object stream.
// indexInStream is the index of the target object within the stream.
func (r *resolver) resolveCompressed(objNum, objStreamNum, indexInStream int) (core.PdfObject, error) {
	// First, resolve the object stream itself.
	streamObj, err := r.Resolve(objStreamNum)
	if err != nil {
		return nil, fmt.Errorf("reader: resolve object stream %d: %w", objStreamNum, err)
	}
	stream, ok := streamObj.(*core.PdfStream)
	if !ok {
		return nil, fmt.Errorf("reader: object stream %d is not a stream (type %T)", objStreamNum, streamObj)
	}

	// Get /N (number of objects in the stream) and /First (offset to first object data).
	nObj := 0
	firstOffset := 0
	if n := stream.Dict.Get("N"); n != nil {
		if num, ok := n.(*core.PdfNumber); ok {
			nObj = num.IntValue()
		}
	}
	if f := stream.Dict.Get("First"); f != nil {
		if num, ok := f.(*core.PdfNumber); ok {
			firstOffset = num.IntValue()
		}
	}

	if nObj <= 0 || firstOffset <= 0 {
		return nil, fmt.Errorf("reader: object stream %d has invalid /N (%d) or /First (%d)", objStreamNum, nObj, firstOffset)
	}

	// The stream data starts with N pairs of (objNum offset) integers,
	// followed by the actual object data starting at /First.
	streamData := stream.Data

	// Bound /N to prevent excessive allocation from a malicious PDF.
	// Each entry in the header requires at least 2 tokens (objNum + offset),
	// each needing at least 2 bytes (digit + separator). Cap at streamData/2.
	maxEntries := len(streamData) / 2
	if maxEntries < 1 {
		maxEntries = 1
	}
	if nObj > maxEntries {
		return nil, fmt.Errorf("reader: object stream %d: /N (%d) exceeds reasonable limit for stream size (%d bytes)", objStreamNum, nObj, len(streamData))
	}

	tok := NewTokenizer(streamData)

	// Read the N pairs of (objNum, offset).
	type objEntry struct {
		objNum int
		offset int
	}
	entries := make([]objEntry, nObj)
	for i := range nObj {
		numTok := tok.Next()
		offTok := tok.Next()
		if numTok.Type != TokenNumber || offTok.Type != TokenNumber {
			return nil, fmt.Errorf("reader: object stream %d: invalid header at entry %d", objStreamNum, i)
		}
		entries[i] = objEntry{
			objNum: int(numTok.Int),
			offset: int(offTok.Int),
		}
	}

	if indexInStream >= len(entries) {
		return nil, fmt.Errorf("reader: object stream %d: index %d out of range (N=%d)", objStreamNum, indexInStream, nObj)
	}

	// Parse the object at the given index.
	objOffset := firstOffset + entries[indexInStream].offset
	if objOffset < 0 || objOffset >= len(streamData) {
		return nil, fmt.Errorf("reader: object stream %d: computed offset %d out of bounds (stream size %d)", objStreamNum, objOffset, len(streamData))
	}
	tok.SetPos(objOffset)
	parser := NewParser(tok)
	obj, err := parser.ParseObject()
	if err != nil {
		return nil, fmt.Errorf("reader: object stream %d, object %d: %w", objStreamNum, objNum, err)
	}

	r.cacheObject(objNum, obj)
	return obj, nil
}

// ResolveRef resolves a PdfIndirectReference to its target object.
func (r *resolver) ResolveRef(ref *core.PdfIndirectReference) (core.PdfObject, error) {
	return r.Resolve(ref.Num())
}

// ResolveDeep resolves an object, following indirect references.
// If obj is a PdfIndirectReference, it resolves it. Otherwise returns obj as-is.
func (r *resolver) ResolveDeep(obj core.PdfObject) (core.PdfObject, error) {
	if ref, ok := obj.(*core.PdfIndirectReference); ok {
		return r.Resolve(ref.Num())
	}
	return obj, nil
}

// resolveStream reads and optionally decompresses stream data.
func (r *resolver) resolveStream(stream *core.PdfStream, objOffset int64) (*core.PdfStream, error) {
	// Resolve /Length if it's an indirect reference.
	lengthObj := stream.Dict.Get("Length")
	streamLen := 0
	if lengthObj != nil {
		resolved, err := r.ResolveDeep(lengthObj)
		if err == nil {
			if num, ok := resolved.(*core.PdfNumber); ok {
				streamLen = num.IntValue()
			}
		}
	}

	// Validate /Length before allocating. A malicious PDF could claim a
	// multi-GB length to trigger an OOM even before decompression.
	if streamLen < 0 {
		return nil, fmt.Errorf("reader: stream has negative /Length %d", streamLen)
	}
	maxRaw := r.mem.limits.effectiveMaxStreamSize()
	if maxRaw >= 0 && int64(streamLen) > maxRaw {
		return nil, fmt.Errorf("%w: raw stream /Length %d exceeds limit %d", ErrMemoryLimitExceeded, streamLen, maxRaw)
	}

	// Find the stream data in the file.
	// The stream data starts after "stream\n" (or "stream\r\n").
	tok := NewTokenizer(r.data)
	tok.SetPos(int(objOffset))

	// Skip past the object header and dictionary to find "stream".
	for tok.pos < tok.len-6 {
		if string(tok.data[tok.pos:tok.pos+6]) == "stream" {
			tok.pos += 6
			// Skip EOL.
			if tok.pos < tok.len && tok.data[tok.pos] == '\r' {
				tok.pos++
			}
			if tok.pos < tok.len && tok.data[tok.pos] == '\n' {
				tok.pos++
			}
			break
		}
		tok.pos++
	}

	streamStart := tok.pos

	if streamLen > 0 && tok.pos+streamLen <= tok.len {
		// Verify that "endstream" follows at the expected position.
		// If it doesn't and we're in tolerant mode, scan for it.
		endPos := tok.pos + streamLen
		endstreamFound := false
		if endPos+9 <= tok.len {
			// Skip optional whitespace/EOL between data and "endstream".
			checkPos := endPos
			for checkPos < tok.len && (tok.data[checkPos] == '\r' || tok.data[checkPos] == '\n' || tok.data[checkPos] == ' ') {
				checkPos++
			}
			if checkPos+9 <= tok.len && string(tok.data[checkPos:checkPos+9]) == "endstream" {
				endstreamFound = true
			}
		}

		if !endstreamFound && r.strictness != StrictnessStrict {
			// /Length appears wrong. Scan forward for "endstream" to find the real length.
			const maxScanDist = 10 * 1024 * 1024 // 10 MB
			actual := scanForEndstream(tok.data, streamStart, maxScanDist)
			if actual >= 0 {
				streamLen = actual - streamStart
				// Re-validate corrected length against memory limits.
				if streamLen < 0 {
					streamLen = 0
				}
				if maxRaw >= 0 && int64(streamLen) > maxRaw {
					return nil, fmt.Errorf("%w: corrected stream /Length %d exceeds limit %d", ErrMemoryLimitExceeded, streamLen, maxRaw)
				}
			}
			// If not found, keep the original /Length value.
		}

		if streamLen > 0 && tok.pos+streamLen <= tok.len {
			rawData := make([]byte, streamLen)
			copy(rawData, tok.data[tok.pos:tok.pos+streamLen])

			// Decompress if needed.
			data, err := decompressStreamLimited(rawData, stream.Dict, r.mem)
			if err != nil {
				return nil, fmt.Errorf("reader: decompress stream: %w", err)
			}

			// If decompression changed the data, the old /Filter and
			// /DecodeParms are stale — skip them and re-compress with
			// FlateDecode on write. If data is unchanged (unknown filter
			// like DCTDecode), preserve the original dictionary as-is.
			decompressed := len(data) != len(rawData) || !bytes.Equal(data, rawData)
			var result *core.PdfStream
			if decompressed {
				result = core.NewPdfStreamCompressed(data)
				for key, value := range stream.Dict.All() {
					switch key {
					case "Filter", "DecodeParms", "Length":
						continue
					default:
						result.Dict.Set(key, value)
					}
				}
			} else {
				// Unknown filter — preserve raw data and original dict.
				result = core.NewPdfStream(rawData)
				for key, value := range stream.Dict.All() {
					if key == "Length" {
						continue // WriteTo recalculates Length
					}
					result.Dict.Set(key, value)
				}
			}
			return result, nil
		}
	}

	return stream, nil
}

// scanForEndstream searches for the "endstream" keyword starting from
// position start in data, scanning up to maxScan bytes.
// Returns the byte offset of "endstream" or -1 if not found.
func scanForEndstream(data []byte, start, maxScan int) int {
	end := start + maxScan
	if end > len(data) {
		end = len(data)
	}
	if start < 0 || start >= len(data) {
		return -1
	}
	idx := bytes.Index(data[start:end], []byte("endstream"))
	if idx < 0 {
		return -1
	}
	pos := start + idx
	// Strip trailing whitespace/EOL before "endstream" to get the actual data end.
	dataEnd := pos
	for dataEnd > start && (data[dataEnd-1] == '\r' || data[dataEnd-1] == '\n') {
		dataEnd--
	}
	return dataEnd
}

// decompressStreamLimited decompresses stream data with memory tracking.
func decompressStreamLimited(data []byte, dict *core.PdfDictionary, mem *memoryTracker) ([]byte, error) {
	maxStream := mem.limits.effectiveMaxStreamSize()
	result, err := decompressStreamWithLimit(data, dict, maxStream)
	if err != nil {
		return nil, err
	}
	if err := mem.checkStreamSize(int64(len(result))); err != nil {
		return nil, err
	}
	return result, nil
}

// decompressStreamWithLimit decompresses stream data with an optional size limit.
// maxBytes < 0 means no limit.
func decompressStreamWithLimit(data []byte, dict *core.PdfDictionary, maxBytes int64) ([]byte, error) {
	filterObj := dict.Get("Filter")
	if filterObj == nil {
		return data, nil // no compression
	}

	// /Filter can be a name or an array of names.
	filters := extractFilters(filterObj)

	result := data
	for _, filter := range filters {
		var err error
		switch filter {
		case "FlateDecode":
			result, err = inflateFlateDecode(result, maxBytes)
		case "ASCIIHexDecode":
			result, err = decodeASCIIHex(result, maxBytes)
		case "ASCII85Decode":
			result, err = decodeASCII85(result, maxBytes)
		default:
			// Unknown filter — return raw data.
			return data, nil
		}
		if err != nil {
			return nil, err
		}
	}

	// Apply predictor if specified in /DecodeParms.
	decodeParms := dict.Get("DecodeParms")
	if decodeParms != nil {
		if parmsDict, ok := decodeParms.(*core.PdfDictionary); ok {
			result, _ = applyPredictor(result, parmsDict)
		}
	}

	return result, nil
}

// applyPredictor reverses PNG/TIFF prediction on decompressed data
// (ISO 32000-1 §7.4.4.4, Table 8). This is commonly used with
// FlateDecode in xref streams and image data.
//
// The row geometry comes from /Colors, /BitsPerComponent and /Columns:
// a row holds Columns samples of Colors components, packed into
// ceil(Colors*BitsPerComponent*Columns/8) bytes. PNG filters look back
// one whole pixel, ceil(Colors*BitsPerComponent/8) bytes, for the left
// neighbor.
//
// The behavior matches PdfPig 0.1.8 (Filters/FlateFilter.cs and
// Filters/PngPredictor.cs), which FulcrumProduct's .NET code uses:
//   - /Colors is capped at 32.
//   - A filter type byte other than 0-4 leaves its row as read.
//   - A truncated final row comes out as a full row. The bytes that the
//     stream does not supply keep the value that the previous decoded
//     row has at that position. Then the row filter runs over the full
//     row.
//
// Parameters that cannot describe a row return data unchanged: Colors
// or Columns less than 1, a BitsPerComponent other than 1, 2, 4, 8 or
// 16, or one row longer than the input. As a result, the output is less
// than 2*len(data). PdfPig gives other results for these values: an
// empty output, the raw stream after an exception, or one zero-padded
// row as long as /Columns asks for.
func applyPredictor(data []byte, parms *core.PdfDictionary) ([]byte, error) {
	predictor := predictorParam(parms, "Predictor", 1)
	isPNG := predictor >= 10 && predictor <= 15
	if !isPNG && predictor != 2 {
		return data, nil // 1 is no prediction; other values are undefined.
	}

	colors := min(predictorParam(parms, "Colors", 1), 32)
	bpc := predictorParam(parms, "BitsPerComponent", 8)
	columns := predictorParam(parms, "Columns", 1)
	if colors < 1 || columns < 1 || len(data) == 0 {
		return data, nil
	}
	switch bpc {
	case 1, 2, 4, 8, 16:
	default:
		return data, nil
	}

	// A row has at least one bit per column, so reject a huge /Columns
	// before the multiplication below can overflow.
	if int64(columns) > int64(len(data))*8 {
		return data, nil
	}
	bitsPerPixel := colors * bpc
	rowLen64 := (int64(columns)*int64(bitsPerPixel) + 7) / 8
	header := int64(0)
	if isPNG {
		header = 1 // the per-row filter type byte
	}
	if rowLen64+header > int64(len(data)) {
		return data, nil
	}
	rowLen := int(rowLen64)

	if isPNG {
		bpp := (bitsPerPixel + 7) / 8
		return decodePNGPredictor(data, rowLen, bpp), nil
	}
	return decodeTIFFPredictor(data, rowLen, colors, bpc, columns), nil
}

// predictorParam reads an integer entry from /DecodeParms. A missing or
// non-numeric entry gives def.
func predictorParam(parms *core.PdfDictionary, key string, def int) int {
	if num, ok := parms.Get(key).(*core.PdfNumber); ok {
		return num.IntValue()
	}
	return def
}

// predictorRows splits data into rows of stride bytes and returns the
// output buffer, which has one rowLen-byte row for each input row. The
// last input row can be short. applyPredictor makes sure that the first
// row is full.
func predictorRows(data []byte, stride, rowLen int) (nRows int, out []byte) {
	nRows = (len(data) + stride - 1) / stride
	return nRows, make([]byte, nRows*rowLen)
}

// decodePNGPredictor reverses PNG row filtering (PNG 1.2 §6). Each input
// row is a filter type byte followed by rowLen data bytes. bpp is the
// distance in bytes to the left neighbor.
func decodePNGPredictor(data []byte, rowLen, bpp int) []byte {
	stride := rowLen + 1
	nRows, out := predictorRows(data, stride, rowLen)
	zero := make([]byte, rowLen)

	for r := range nRows {
		in := data[r*stride : min((r+1)*stride, len(data))]
		row := out[r*rowLen : (r+1)*rowLen]
		prev := zero
		if r > 0 {
			prev = out[(r-1)*rowLen : r*rowLen]
		}
		raw := in[1:]
		if len(raw) < rowLen {
			// A short final row keeps the previous row's bytes past its
			// end, as in PdfPig, which reads into an uncleared buffer.
			copy(row, prev)
		}
		copy(row, raw)

		switch in[0] {
		case 1: // Sub
			for i := bpp; i < rowLen; i++ {
				row[i] += row[i-bpp]
			}
		case 2: // Up
			for i := range rowLen {
				row[i] += prev[i]
			}
		case 3: // Average
			for i := range rowLen {
				left := 0
				if i >= bpp {
					left = int(row[i-bpp])
				}
				row[i] += byte((left + int(prev[i])) / 2)
			}
		case 4: // Paeth
			for i := range rowLen {
				var left, upLeft byte
				if i >= bpp {
					left = row[i-bpp]
					upLeft = prev[i-bpp]
				}
				row[i] += paethPredictor(left, prev[i], upLeft)
			}
		}
		// 0 (None) and unknown types leave the row as read.
	}
	return out
}

// decodeTIFFPredictor reverses TIFF Predictor 2 (horizontal
// differencing, TIFF 6.0 §14). Each sample component is stored as the
// difference from the same component of the pixel to its left, modulo
// 2^bpc.
func decodeTIFFPredictor(data []byte, rowLen, colors, bpc, columns int) []byte {
	nRows, out := predictorRows(data, rowLen, rowLen)
	samples := columns * colors
	if bpc == 1 && colors == 1 {
		// PdfPig runs the 1-bit case over every bit of the row,
		// including the padding bits after the last column.
		samples = rowLen * 8
	}

	for r := range nRows {
		row := out[r*rowLen : (r+1)*rowLen]
		in := data[r*rowLen : min((r+1)*rowLen, len(data))]
		if len(in) < rowLen && r > 0 {
			copy(row, out[(r-1)*rowLen:r*rowLen]) // as in the PNG case
		}
		copy(row, in)

		switch bpc {
		case 8:
			for i := colors; i < samples; i++ {
				row[i] += row[i-colors]
			}
		case 16:
			for i := colors; i < samples; i++ {
				p, l := 2*i, 2*(i-colors)
				v := uint16(row[p])<<8 | uint16(row[p+1])
				v += uint16(row[l])<<8 | uint16(row[l+1])
				row[p], row[p+1] = byte(v>>8), byte(v)
			}
		default: // 1, 2 or 4 bits: samples never cross a byte.
			mask := byte(1)<<bpc - 1
			shift := func(i int) (int, int) {
				bit := i * bpc
				return bit / 8, 8 - bit%8 - bpc
			}
			for i := colors; i < samples; i++ {
				pb, ps := shift(i)
				lb, ls := shift(i - colors)
				v := (row[pb]>>ps + row[lb]>>ls) & mask
				row[pb] = row[pb]&^(mask<<ps) | v<<ps
			}
		}
	}
	return out
}

// paethPredictor computes the Paeth predictor value.
func paethPredictor(a, b, c byte) byte {
	p := int(a) + int(b) - int(c)
	pa := abs(p - int(a))
	pb := abs(p - int(b))
	pc := abs(p - int(c))
	if pa <= pb && pa <= pc {
		return a
	}
	if pb <= pc {
		return b
	}
	return c
}

// abs returns the absolute value of x.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// extractFilters gets the filter name(s) from a /Filter value.
func extractFilters(obj core.PdfObject) []string {
	if name, ok := obj.(*core.PdfName); ok {
		return []string{name.Value}
	}
	if arr, ok := obj.(*core.PdfArray); ok {
		var filters []string
		for _, elem := range arr.All() {
			if name, ok := elem.(*core.PdfName); ok {
				filters = append(filters, name.Value)
			}
		}
		return filters
	}
	return nil
}

// inflateFlateDecode decompresses zlib-compressed data.
// maxBytes limits the decompressed output size (-1 = unlimited).
func inflateFlateDecode(data []byte, maxBytes int64) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("reader: FlateDecode: %w", err)
	}
	defer func() { _ = r.Close() }()

	result, err := limitedReadAll(r, maxBytes)
	if err != nil {
		return nil, fmt.Errorf("reader: FlateDecode: %w", err)
	}
	return result, nil
}

// decodeASCIIHex decodes ASCIIHexDecode filter data.
// maxBytes limits the decoded output size (-1 = unlimited).
func decodeASCIIHex(data []byte, maxBytes int64) ([]byte, error) {
	var hex []byte
	for _, b := range data {
		if b == '>' {
			break
		}
		if !isWhitespace(b) {
			hex = append(hex, b)
		}
	}
	if len(hex)%2 != 0 {
		hex = append(hex, '0')
	}
	outLen := int64(len(hex) / 2)
	if maxBytes >= 0 && outLen > maxBytes {
		return nil, fmt.Errorf("%w: ASCIIHexDecode output %d exceeds limit %d", ErrMemoryLimitExceeded, outLen, maxBytes)
	}
	result := make([]byte, outLen)
	for i := 0; i < len(hex); i += 2 {
		result[i/2] = hexVal(hex[i])<<4 | hexVal(hex[i+1])
	}
	return result, nil
}

// decodeASCII85 decodes ASCII85/btoa encoded data.
// maxBytes limits the decoded output size (-1 = unlimited).
func decodeASCII85(data []byte, maxBytes int64) ([]byte, error) {
	var result []byte
	var group [5]byte
	n := 0

	checkLimit := func() error {
		if maxBytes >= 0 && int64(len(result)) > maxBytes {
			return fmt.Errorf("%w: ASCII85Decode output exceeds limit %d", ErrMemoryLimitExceeded, maxBytes)
		}
		return nil
	}

	for _, b := range data {
		if b == '~' {
			break // end marker ~>
		}
		if isWhitespace(b) {
			continue
		}
		if b == 'z' && n == 0 {
			result = append(result, 0, 0, 0, 0)
			if err := checkLimit(); err != nil {
				return nil, err
			}
			continue
		}
		group[n] = b - 33
		n++
		if n == 5 {
			val := uint32(group[0])*85*85*85*85 +
				uint32(group[1])*85*85*85 +
				uint32(group[2])*85*85 +
				uint32(group[3])*85 +
				uint32(group[4])
			result = append(result, byte(val>>24), byte(val>>16), byte(val>>8), byte(val))
			n = 0
			if err := checkLimit(); err != nil {
				return nil, err
			}
		}
	}

	// Handle remaining bytes.
	if n > 1 {
		for i := n; i < 5; i++ {
			group[i] = 84 // pad with 'u' (84 = 'u' - 33)
		}
		val := uint32(group[0])*85*85*85*85 +
			uint32(group[1])*85*85*85 +
			uint32(group[2])*85*85 +
			uint32(group[3])*85 +
			uint32(group[4])
		for i := range n - 1 {
			result = append(result, byte(val>>(24-8*i)))
		}
	}

	return result, nil
}
