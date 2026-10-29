package editor

import (
	_ "embed"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sync"
	"unicode/utf8"
)

// Thai uses ICU4X's codepoint LSTM rather than standalone UAX #29 boundaries.
// The payload is generated from icu_segmenter 2.4.0
// SegmenterLstmAutoV1/Thai_codepoints_exclusive_model4_heavy (Unicode 18).
//
//go:embed segmentation_thai_lstm.bin
var thaiLSTMBytes []byte

type thaiLSTMBoundary struct {
	start int
	end   int
}

type thaiLSTM struct {
	ids       [128]int16
	unknown   uint16
	hunits    int
	embedding int

	emb   []float32
	fwW   []float32
	fwU   []float32
	fwB   []float32
	bwW   []float32
	bwU   []float32
	bwB   []float32
	timeW []float32
	timeB []float32
}

var loadThaiLSTM = sync.OnceValue(func() *thaiLSTM {
	model, err := decodeThaiLSTM(thaiLSTMBytes)
	if err != nil {
		panic(fmt.Sprintf("decode embedded Thai segmentation model: %v", err))
	}
	return model
})

func decodeThaiLSTM(data []byte) (*thaiLSTM, error) {
	const magic = "ICUTHAI1"
	if len(data) < len(magic) || string(data[:len(magic)]) != magic {
		return nil, errors.New("invalid model header")
	}
	offset := len(magic)
	count, ok := readUint16(data, &offset)
	if !ok || count == 0 || count > 128 {
		return nil, errors.New("invalid dictionary size")
	}
	hunits, ok := readUint16(data, &offset)
	if !ok || hunits == 0 {
		return nil, errors.New("invalid hidden size")
	}
	embedding, ok := readUint16(data, &offset)
	if !ok || embedding == 0 {
		return nil, errors.New("invalid embedding size")
	}

	model := &thaiLSTM{
		unknown:   count,
		hunits:    int(hunits),
		embedding: int(embedding),
	}
	for index := range model.ids {
		model.ids[index] = -1
	}
	for range count {
		length, ok := readUint16(data, &offset)
		if !ok || length == 0 || offset+int(length)+2 > len(data) {
			return nil, errors.New("invalid dictionary entry")
		}
		key := data[offset : offset+int(length)]
		offset += int(length)
		r, size := utf8.DecodeRune(key)
		if r == utf8.RuneError || size != len(key) {
			return nil, errors.New("dictionary key is not one UTF-8 rune")
		}
		id, ok := readUint16(data, &offset)
		if !ok || id >= count {
			return nil, errors.New("invalid dictionary id")
		}
		slot := int(r - '\u0e00')
		if slot < 0 || slot >= len(model.ids) || model.ids[slot] != -1 {
			return nil, errors.New("dictionary rune is out of range or duplicated")
		}
		model.ids[slot] = int16(id)
	}

	h := model.hunits
	e := model.embedding
	var err error
	if model.emb, err = readFloat32s(data, &offset, (int(count)+1)*e); err != nil {
		return nil, err
	}
	if model.fwW, err = readFloat32s(data, &offset, 4*h*e); err != nil {
		return nil, err
	}
	if model.fwU, err = readFloat32s(data, &offset, 4*h*h); err != nil {
		return nil, err
	}
	if model.fwB, err = readFloat32s(data, &offset, 4*h); err != nil {
		return nil, err
	}
	if model.bwW, err = readFloat32s(data, &offset, 4*h*e); err != nil {
		return nil, err
	}
	if model.bwU, err = readFloat32s(data, &offset, 4*h*h); err != nil {
		return nil, err
	}
	if model.bwB, err = readFloat32s(data, &offset, 4*h); err != nil {
		return nil, err
	}
	if model.timeW, err = readFloat32s(data, &offset, 2*4*h); err != nil {
		return nil, err
	}
	if model.timeB, err = readFloat32s(data, &offset, 4); err != nil {
		return nil, err
	}
	if offset != len(data) {
		return nil, errors.New("trailing model data")
	}
	return model, nil
}

func readUint16(data []byte, offset *int) (uint16, bool) {
	if *offset+2 > len(data) {
		return 0, false
	}
	value := binary.LittleEndian.Uint16(data[*offset : *offset+2])
	*offset += 2
	return value, true
}

func readFloat32s(data []byte, offset *int, count int) ([]float32, error) {
	if count < 0 || *offset+count*4 > len(data) {
		return nil, errors.New("truncated model matrix")
	}
	values := make([]float32, count)
	for index := range values {
		at := *offset + index*4
		values[index] = math.Float32frombits(binary.LittleEndian.Uint32(data[at : at+4]))
	}
	*offset += count * 4
	return values, nil
}

func (m *thaiLSTM) segment(text string) []thaiLSTMBoundary {
	ids := make([]uint16, 0, utf8.RuneCountInString(text))
	positions := make([]int, 1, len(ids)+1)
	for at, r := range text {
		ids = append(ids, m.id(r))
		positions = append(positions, at+utf8.RuneLen(r))
	}
	if len(ids) == 0 {
		return nil
	}

	h := m.hunits
	backwardHidden := make([]float32, len(ids)*h)
	backwardCell := make([]float32, h)
	scratch := make([]float32, 4*h)
	for index := len(ids) - 1; index >= 0; index-- {
		if index+1 < len(ids) {
			copy(backwardHidden[index*h:(index+1)*h], backwardHidden[(index+1)*h:(index+2)*h])
		}
		m.computeHiddenCell(
			m.embeddingRow(ids[index]),
			backwardHidden[index*h:(index+1)*h],
			backwardCell,
			m.bwW,
			m.bwU,
			m.bwB,
			scratch,
		)
	}

	forwardHidden := make([]float32, h)
	forwardCell := make([]float32, h)
	boundaries := make([]thaiLSTMBoundary, 0, 4)
	start := 0
	for index, id := range ids {
		m.computeHiddenCell(
			m.embeddingRow(id),
			forwardHidden,
			forwardCell,
			m.fwW,
			m.fwU,
			m.fwB,
			scratch,
		)
		backward := backwardHidden[index*h : (index+1)*h]
		var weights [4]float32
		for class := range weights {
			weights[class] = dotProduct(
				forwardHidden,
				m.timeW[(class*h):(class+1)*h],
			)
			weights[class] += dotProduct(
				backward,
				m.timeW[(4+class)*h:(4+class+1)*h],
			)
			weights[class] += m.timeB[class]
		}
		if index == len(ids)-1 || weights[2] > weights[0] &&
			weights[2] > weights[1] &&
			weights[2] > weights[3] {
			boundaries = append(boundaries, thaiLSTMBoundary{
				start: positions[start],
				end:   positions[index+1],
			})
			start = index + 1
		}
	}
	return boundaries
}

func (m *thaiLSTM) embeddingRow(id uint16) []float32 {
	start := int(id) * m.embedding
	return m.emb[start : start+m.embedding]
}

func (m *thaiLSTM) id(r rune) uint16 {
	slot := int(r - '\u0e00')
	if slot < 0 || slot >= len(m.ids) || m.ids[slot] < 0 {
		return m.unknown
	}
	return uint16(m.ids[slot])
}

func (m *thaiLSTM) computeHiddenCell(
	input []float32,
	hidden []float32,
	cell []float32,
	weight []float32,
	recurrent []float32,
	bias []float32,
	state []float32,
) {
	h := m.hunits
	e := m.embedding
	copy(state, bias)
	for class := range 4 {
		for unit := range h {
			at := class*h + unit
			state[at] += dotProduct(input, weight[at*e:(at+1)*e])
			state[at] += dotProduct(hidden, recurrent[at*h:(at+1)*h])
		}
	}
	for unit := range h {
		state[unit] = sigmoid(state[unit])
		state[h+unit] = sigmoid(state[h+unit])
		state[2*h+unit] = tanh(state[2*h+unit])
		state[3*h+unit] = sigmoid(state[3*h+unit])
	}
	for unit := range h {
		cell[unit] = state[unit]*state[2*h+unit] + cell[unit]*state[h+unit]
		hidden[unit] = state[3*h+unit] * tanh(cell[unit])
	}
}

// dotProduct preserves ICU4X's eight-lane accumulation order.
func dotProduct(left, right []float32) float32 {
	var lanes [8]float32
	chunks := len(left) / len(lanes)
	for chunk := range chunks {
		base := chunk * len(lanes)
		for lane := range lanes {
			lanes[lane] += left[base+lane] * right[base+lane]
		}
	}
	var tail float32
	for index := chunks * len(lanes); index < len(left); index++ {
		tail += left[index] * right[index]
	}
	return tail +
		(lanes[0] + lanes[4]) +
		(lanes[1] + lanes[5]) +
		(lanes[2] + lanes[6]) +
		(lanes[3] + lanes[7])
}

func sigmoid(value float32) float32 {
	return 1 / (1 + float32(math.Exp(float64(-value))))
}

func tanh(value float32) float32 {
	return float32(math.Tanh(float64(value)))
}
