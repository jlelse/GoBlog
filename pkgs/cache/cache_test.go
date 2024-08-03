package cache

import (
	"sync"
	"testing"
	"time"
)

func TestGetSet(t *testing.T) {
	cycle := 100 * time.Millisecond
	c := New[string, string](cycle, 100)
	defer c.Close()

	c.Set("sticky", "forever", 0, 1)
	c.Set("hello", "Hello", cycle/2, 1)

	hello, found := c.Get("hello")
	if !found {
		t.FailNow()
	}
	if hello != "Hello" {
		t.FailNow()
	}

	time.Sleep(cycle / 2)

	_, found = c.Get("hello")
	if found {
		t.FailNow()
	}

	time.Sleep(cycle)

	_, found = c.Get("404")
	if found {
		t.FailNow()
	}

	_, found = c.Get("sticky")
	if !found {
		t.FailNow()
	}
}

func TestDelete(t *testing.T) {
	c := New[string, string](time.Minute, 100)
	c.Set("hello", "Hello", time.Hour, 1)

	_, found := c.Get("hello")
	if !found {
		t.FailNow()
	}

	c.Delete("hello")

	_, found = c.Get("hello")
	if found {
		t.FailNow()
	}
}

func TestLRUEviction(t *testing.T) {
	c := New[string, string](time.Minute, 10)
	defer c.Close()

	for i := 0; i < 10; i++ {
		c.Set(string(rune('a'+i)), string(rune('A'+i)), 0, 1)
	}

	// Access first item to make it recently used
	c.Get("a")

	// Add another item to trigger eviction
	c.Set("k", "K", 0, 1)

	// Ensure 'a' is still there
	if _, found := c.Get("a"); !found {
		t.Error("expected 'a' to be found")
	}

	// Ensure 'b' is evicted
	if _, found := c.Get("b"); found {
		t.Error("expected 'b' to be evicted")
	}
}

func TestCostEviction(t *testing.T) {
	c := New[string, string](time.Minute, 10)
	defer c.Close()

	// Add items with varying costs
	c.Set("a", "A", 0, 3)
	c.Set("b", "B", 0, 3)
	c.Set("c", "C", 0, 3)

	// This should evict 'a'
	c.Set("d", "D", 0, 4)

	if _, found := c.Get("a"); found {
		t.Error("expected 'a' to be evicted")
	}

	if _, found := c.Get("b"); !found {
		t.Error("expected 'b' to be found")
	}

	if _, found := c.Get("c"); !found {
		t.Error("expected 'c' to be found")
	}

	if _, found := c.Get("d"); !found {
		t.Error("expected 'd' to be found")
	}
}

func TestTimeEviction(t *testing.T) {
	c := New[string, string](100*time.Millisecond, 10)
	defer c.Close()

	c.Set("a", "A", 10*time.Millisecond, 1)

	time.Sleep(200 * time.Millisecond)

	if _, found := c.Get("a"); found {
		t.Error("expected 'a' to be evicted")
	}
}

func BenchmarkNew(b *testing.B) {
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			New[string, string](5*time.Second, 100).Close()
		}
	})
}

func BenchmarkGet(b *testing.B) {
	c := New[string, string](5*time.Second, 100)
	defer c.Close()

	c.Set("Hello", "World", 0, 1)

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Get("Hello")
		}
	})
}

func BenchmarkSet(b *testing.B) {
	c := New[string, string](5*time.Second, 100)
	defer c.Close()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Set("Hello", "World", 0, 1)
		}
	})
}

func BenchmarkDelete(b *testing.B) {
	c := New[string, string](5*time.Second, 100)
	defer c.Close()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Delete("Hello")
		}
	})
}

func TestConcurrentSetGet(t *testing.T) {
	c := New[string, string](time.Minute, 100)
	defer c.Close()

	var wg sync.WaitGroup

	set := func(k, v string) {
		defer wg.Done()
		c.Set(k, v, time.Hour, 1)
	}

	get := func(k string) {
		defer wg.Done()
		c.Get(k)
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go set(string(rune('a'+i)), string(rune('A'+i)))
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go get(string(rune('a' + i)))
	}

	wg.Wait()
}

func TestConcurrentSetDelete(t *testing.T) {
	c := New[string, string](time.Minute, 100)
	defer c.Close()

	var wg sync.WaitGroup

	set := func(k, v string) {
		defer wg.Done()
		c.Set(k, v, time.Hour, 1)
	}

	delete := func(k string) {
		defer wg.Done()
		c.Delete(k)
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go set(string(rune('a'+i)), string(rune('A'+i)))
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go delete(string(rune('a' + i)))
	}

	wg.Wait()
}

func TestConcurrentSetGetDelete(t *testing.T) {
	c := New[string, string](time.Minute, 100)
	defer c.Close()

	var wg sync.WaitGroup

	set := func(k, v string) {
		defer wg.Done()
		c.Set(k, v, time.Hour, 1)
	}

	get := func(k string) {
		defer wg.Done()
		c.Get(k)
	}

	delete := func(k string) {
		defer wg.Done()
		c.Delete(k)
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go set(string(rune('a'+i)), string(rune('A'+i)))
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go get(string(rune('a' + i)))
	}

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go delete(string(rune('a' + i)))
	}

	wg.Wait()
}

func TestNegativeCost(t *testing.T) {
	c := New[string, string](time.Minute, 10)
	defer c.Close()

	c.Set("neg", "Negative", 0, -5)

	_, found := c.Get("neg")
	if !found {
		t.Error("expected 'neg' to be found")
	}
}

func TestZeroDuration(t *testing.T) {
	c := New[string, string](time.Minute, 10)
	defer c.Close()

	c.Set("zero", "Zero", 0, 1)

	_, found := c.Get("zero")
	if !found {
		t.Error("expected 'zero' to be found")
	}
}

func TestClear(t *testing.T) {
	c := New[string, string](time.Minute, 10)
	defer c.Close()

	c.Set("a", "A", 0, 1)
	c.Set("b", "B", 0, 1)

	c.Clear()

	if _, found := c.Get("a"); found {
		t.Error("expected 'a' to be cleared")
	}

	if _, found := c.Get("b"); found {
		t.Error("expected 'b' to be cleared")
	}
}

func TestSameKey(t *testing.T) {
	c := New[string, string](time.Minute, 10)
	defer c.Close()

	c.Set("a", "A", 0, 1)
	c.Set("a", "B", 0, 1)

	val, found := c.Get("a")
	if !found {
		t.Error("expected 'a' to be found")
	}

	if val != "B" {
		t.Errorf("expected 'a' to be 'B', got %v", val)
	}
}
