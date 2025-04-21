package builderpool

import (
	"testing"
)

func TestGet(t *testing.T) {
	builder := Get()
	if builder == nil {
		t.Fatal("Expected a non-nil *strings.Builder from Get()")
	}

	// Ensure the builder is empty
	if builder.Len() != 0 {
		t.Fatalf("Expected an empty builder, got length %d", builder.Len())
	}

	// Return the builder to the pool
	Put(builder)
}

func TestPut(t *testing.T) {
	builder := Get()
	builder.WriteString("test data")

	// Return the builder to the pool
	Put(builder)

	// Get the builder back from the pool
	reusedBuilder := Get()
	if reusedBuilder.Len() != 0 {
		t.Fatalf("Expected the builder to be reset, but got length %d", reusedBuilder.Len())
	}

	// Return the builder to the pool again
	Put(reusedBuilder)
}

func TestPutMultiple(t *testing.T) {
	builder1 := Get()
	builder2 := Get()

	builder1.WriteString("data1")
	builder2.WriteString("data2")

	// Return multiple builders to the pool
	Put(builder1, builder2)

	// Get builders back from the pool and ensure they are reset
	reusedBuilder1 := Get()
	reusedBuilder2 := Get()

	if reusedBuilder1.Len() != 0 {
		t.Fatalf("Expected builder1 to be reset, got length %d", reusedBuilder1.Len())
	}
	if reusedBuilder2.Len() != 0 {
		t.Fatalf("Expected builder2 to be reset, got length %d", reusedBuilder2.Len())
	}

	// Return builders to the pool again
	Put(reusedBuilder1, reusedBuilder2)
}
