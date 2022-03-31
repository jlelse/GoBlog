package mp3merge

import (
	"errors"
	"io"

	"github.com/dmulholl/mp3lib"
	"go.goblog.app/app/pkgs/bufferpool"
)

// Inspired by https://github.com/dmulholl/mp3cat/blob/2ec1e4fe4d995ebd41bf1887b3cab8e2a569b3d4/mp3cat.go
// Merge multiple mp3s into one mp3.
func MergeMP3(out io.Writer, in ...io.Reader) error {
	if len(in) == 0 {
		return errors.New("no inputs specified")
	}

	var totalFrames, totalBytes uint32
	var firstBitRate int
	var isVBR bool

	tmpOut := bufferpool.Get()
	defer bufferpool.Put(tmpOut)

	// Loop over the input files and append their MP3 frames to the output file.
	for _, inReader := range in {
		if inReader == nil {
			return errors.New("nil input")
		}

		isFirstFrame := true

		for {
			// Read the next frame from the input
			frame := mp3lib.NextFrame(inReader)
			if frame == nil {
				break
			}

			// Skip the first frame if it's a VBR header
			if isFirstFrame {
				isFirstFrame = false
				if mp3lib.IsXingHeader(frame) || mp3lib.IsVbriHeader(frame) {
					continue
				}
			}

			// If we detect more than one bitrate we'll need to add a VBR header to the output
			if firstBitRate == 0 {
				firstBitRate = frame.BitRate
			} else if frame.BitRate != firstBitRate {
				isVBR = true
			}

			// Write the frame to the temporary output
			_, err := tmpOut.Write(frame.RawBytes)
			if err != nil {
				return err
			}

			// Increment the total number of frames and bytes
			totalFrames += 1
			totalBytes += uint32(len(frame.RawBytes))
		}
	}

	// If we detected multiple bitrates, prepend a VBR header to the output
	if isVBR {
		xingHeader := mp3lib.NewXingHeader(totalFrames, totalBytes)
		_, err := out.Write(xingHeader.RawBytes)
		if err != nil {
			return err
		}
	}

	// Copy the temporary output to the output
	_, err := io.Copy(out, tmpOut)
	return err
}
