package mp3merge

import (
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/dmulholl/mp3lib"
	"github.com/thoas/go-funk"
)

// Inspired by https://github.com/dmulholl/mp3cat/blob/2ec1e4fe4d995ebd41bf1887b3cab8e2a569b3d4/mp3cat.go

// Merge multiple mp3 files into one file
func MergeMP3(out string, in []string) error {

	var totalFrames, totalBytes uint32
	var firstBitRate int
	var isVBR bool

	// Check if output file is included in input files
	if funk.ContainsString(in, out) {
		return errors.New("the list of input files includes the output file")
	}

	// Create the output file.
	if err := os.MkdirAll(filepath.Dir(out), os.ModePerm); err != nil {
		return err
	}
	outfile, err := os.Create(out)
	if err != nil {
		return err
	}

	// Loop over the input files and append their MP3 frames to the output file.
	for _, inpath := range in {
		infile, err := os.Open(inpath)
		if err != nil {
			return err
		}

		isFirstFrame := true

		for {
			// Read the next frame from the input file.
			frame := mp3lib.NextFrame(infile)
			if frame == nil {
				break
			}

			// Skip the first frame if it's a VBR header.
			if isFirstFrame {
				isFirstFrame = false
				if mp3lib.IsXingHeader(frame) || mp3lib.IsVbriHeader(frame) {
					continue
				}
			}

			// If we detect more than one bitrate we'll need to add a VBR
			// header to the output file.
			if firstBitRate == 0 {
				firstBitRate = frame.BitRate
			} else if frame.BitRate != firstBitRate {
				isVBR = true
			}

			// Write the frame to the output file.
			_, err := outfile.Write(frame.RawBytes)
			if err != nil {
				return err
			}

			totalFrames += 1
			totalBytes += uint32(len(frame.RawBytes))
		}

		_ = infile.Close()
	}

	_ = outfile.Close()

	// If we detected multiple bitrates, prepend a VBR header to the file.
	if isVBR {
		err = addXingHeader(out, totalFrames, totalBytes)
		if err != nil {
			return err
		}
	}

	return nil

}

// Prepend an Xing VBR header to the specified MP3 file.
func addXingHeader(filepath string, totalFrames, totalBytes uint32) error {
	tmpSuffix := ".mp3merge.tmp"

	outputFile, err := os.Create(filepath + tmpSuffix)
	if err != nil {
		return err
	}

	inputFile, err := os.Open(filepath)
	if err != nil {
		return err
	}

	xingHeader := mp3lib.NewXingHeader(totalFrames, totalBytes)

	_, err = outputFile.Write(xingHeader.RawBytes)
	if err != nil {
		return err
	}

	_, err = io.Copy(outputFile, inputFile)
	if err != nil {
		return err
	}

	_ = outputFile.Close()
	_ = inputFile.Close()

	err = os.Remove(filepath)
	if err != nil {
		return err
	}

	err = os.Rename(filepath+tmpSuffix, filepath)
	if err != nil {
		return err
	}

	return nil

}
