package bufferpool

import (
	"testing"
)

func TestGet(t *testing.T) {
	buf := Get()
	if buf == nil {
		t.Fatal("Expected non-nil buffer from Get")
	}

	// Ensure the buffer is empty
	if buf.Len() != 0 {
		t.Fatalf("Expected empty buffer, got length %d", buf.Len())
	}

	// Return the buffer to the pool
	Put(buf)
}

func TestPut(t *testing.T) {
	buf := Get()
	buf.WriteString("test data")

	// Return the buffer to the pool
	Put(buf)

	// Get the buffer back from the pool
	reusedBuf := Get()
	if reusedBuf.Len() != 0 {
		t.Fatalf("Expected buffer to be reset, got length %d", reusedBuf.Len())
	}

	// Return the buffer to the pool again
	Put(reusedBuf)
}

func TestPutMultiple(t *testing.T) {
	buf1 := Get()
	buf2 := Get()

	buf1.WriteString("data1")
	buf2.WriteString("data2")

	// Return multiple buffers to the pool
	Put(buf1, buf2)

	// Get buffers back from the pool and ensure they are reset
	reusedBuf1 := Get()
	reusedBuf2 := Get()

	if reusedBuf1.Len() != 0 {
		t.Fatalf("Expected buffer1 to be reset, got length %d", reusedBuf1.Len())
	}
	if reusedBuf2.Len() != 0 {
		t.Fatalf("Expected buffer2 to be reset, got length %d", reusedBuf2.Len())
	}

	// Return buffers to the pool again
	Put(reusedBuf1, reusedBuf2)
}
