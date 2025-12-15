package mp3merge

import (
	"bytes"
	"testing"

	"github.com/dmulholl/mp3lib"
	"github.com/stretchr/testify/require"
)

func TestMergeMP3RequiresInput(t *testing.T) {
	// The merging helper should reject calls that do not supply any readers.
	var out bytes.Buffer
	err := MergeMP3(&out)
	require.EqualError(t, err, "no inputs specified")
}

func TestMergeMP3RejectsNilInput(t *testing.T) {
	// Guard against nil readers so we fail fast instead of panicking mid-merge.
	var out bytes.Buffer
	err := MergeMP3(&out, nil)
	require.EqualError(t, err, "nil input")
}

func TestMergeMP3ConstantBitrate(t *testing.T) {
	// Build two fake MP3 blobs that share the same bitrate to emulate
	// concatenating homogeneous files (the most common case for podcasts).
	first := newFakeMP3(3, bitrateIdx128)
	second := newFakeMP3(2, bitrateIdx128)

	var out bytes.Buffer
	err := MergeMP3(&out, bytes.NewReader(first.data), bytes.NewReader(second.data))
	require.NoError(t, err)

	expected := append(append([]byte{}, first.data...), second.data...)
	require.Equal(t, expected, out.Bytes())

	// Decode the merged bytes with mp3lib to make sure we genuinely produced
	// decodable MP3 frames and did not accidentally emit a header frame.
	reader := bytes.NewReader(out.Bytes())
	var framesRead int
	for {
		frame := mp3lib.NextFrame(reader)
		if frame == nil {
			break
		}
		if framesRead == 0 {
			require.False(t, mp3lib.IsXingHeader(frame))
		}
		require.Equal(t, first.bitRate, frame.BitRate)
		framesRead++
	}
	require.Equal(t, first.frames+second.frames, framesRead)
}

func TestMergeMP3VariableBitrateAddsXingHeader(t *testing.T) {
	// Simulate concatenating recordings that use different bitrates so the
	// merger must create a VBR (Xing) header that advertises mixed bitrates.
	low := newFakeMP3(2, bitrateIdx128)
	high := newFakeMP3(2, bitrateIdx160)

	var out bytes.Buffer
	err := MergeMP3(&out, bytes.NewReader(low.data), bytes.NewReader(high.data))
	require.NoError(t, err)

	output := out.Bytes()
	headReader := bytes.NewReader(output)
	headFrame := mp3lib.NextFrame(headReader)
	require.NotNil(t, headFrame)
	require.True(t, mp3lib.IsXingHeader(headFrame))

	// After the auto-generated header, the payload should be the raw frames
	// from both sources in their original order.
	payload := output[len(headFrame.RawBytes):]
	expectedPayload := append(append([]byte{}, low.data...), high.data...)
	require.Equal(t, expectedPayload, payload)

	payloadReader := bytes.NewReader(payload)
	var bitRates []int
	for {
		frame := mp3lib.NextFrame(payloadReader)
		if frame == nil {
			break
		}
		bitRates = append(bitRates, frame.BitRate)
	}
	require.Equal(t, []int{low.bitRate, low.bitRate, high.bitRate, high.bitRate}, bitRates)
}

type fakeMP3 struct {
	data    []byte
	frames  int
	bitRate int
}

// The constants below encode the few MP3 header fields we need in order to
// produce byte streams that real decoders understand, without shipping full
// sample data in the repository.
const (
	bitrateIdx128          = byte(0x09)
	bitrateIdx160          = byte(0x0A)
	sampleRateIndex44100   = byte(0x00)
	sampleRateValue        = 44100
	frameHeaderSize        = 4
	mpegLayer3SlotMultiple = 144
)

var bitRateIndexToBps = map[byte]int{
	bitrateIdx128: 128000,
	bitrateIdx160: 160000,
}

func newFakeMP3(frameCount int, bitrateIdx byte) fakeMP3 {
	// Each fake MP3 is a sequence of valid frames with predictable lengths so
	// our expectations around total frames and bytes match what MergeMP3 sees.
	bitRate, ok := bitRateIndexToBps[bitrateIdx]
	if !ok {
		panic("unsupported bitrate index")
	}

	frameLen := frameSizeFor(bitrateIdx)
	buf := make([]byte, 0, frameCount*frameLen)
	for i := range frameCount {
		buf = append(buf, buildFrame(bitrateIdx, byte(i))...)
	}

	return fakeMP3{
		data:    buf,
		frames:  frameCount,
		bitRate: bitRate,
	}
}

func buildFrame(bitrateIdx, seed byte) []byte {
	// The payload does not contain real audio samples; we only need unique
	// bytes so the decoder can step from one frame to the next without errors.
	frameLen := frameSizeFor(bitrateIdx)
	head := buildHeader(bitrateIdx, sampleRateIndex44100)
	frame := make([]byte, frameLen)
	copy(frame, head[:])
	for i := frameHeaderSize; i < frameLen; i++ {
		frame[i] = byte((i + int(seed)) & 0xFF)
	}
	return frame
}

func frameSizeFor(bitrateIdx byte) int {
	// MPEG-1 Layer III CBR frames have the deterministic length shown in the
	// standard; replicating it keeps our synthetic frames authentic enough for
	// mp3lib to parse.
	bitRate, ok := bitRateIndexToBps[bitrateIdx]
	if !ok {
		panic("unsupported bitrate index")
	}
	return (mpegLayer3SlotMultiple * bitRate) / sampleRateValue
}

func buildHeader(bitrateIdx, sampleRateIdx byte) [frameHeaderSize]byte {
	// The header structure mirrors the spec (sync word + control bits), which
	// ensures mp3lib will treat the frame as legitimate.
	var header [frameHeaderSize]byte
	header[0] = 0xFF
	header[1] = 0xFB
	header[2] = (bitrateIdx << 4) | (sampleRateIdx << 2)
	header[3] = 0x00
	return header
}
