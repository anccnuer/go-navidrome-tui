package cache

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestGetSetAndLRUEviction(t *testing.T) {
	c := New[int](2, 0)
	c.Set("a", 1)
	c.Set("b", 2)
	if _, ok := c.Get("a"); !ok { // a becomes most recently used
		t.Fatal("a should be cached")
	}
	c.Set("c", 3) // evicts b
	if _, ok := c.Get("b"); ok {
		t.Fatal("b should have been evicted")
	}
	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Fatalf("a = %v, %v; want 1, true", v, ok)
	}
}

func TestTTLExpiry(t *testing.T) {
	now := time.Unix(1000, 0)
	c := New[string](0, 5*time.Second)
	c.now = func() time.Time { return now }
	c.Set("k", "v")
	if _, ok := c.Get("k"); !ok {
		t.Fatal("entry should be fresh")
	}
	now = now.Add(6 * time.Second)
	if _, ok := c.Get("k"); ok {
		t.Fatal("entry should have expired")
	}
	if n := c.Len(); n != 0 {
		t.Fatalf("len = %d, want 0", n)
	}
}

func TestInvalidatePrefixAndClear(t *testing.T) {
	c := New[int](0, 0)
	for _, k := range []string{"album:1", "album:2", "artist:1", "song:1"} {
		c.Set(k, 1)
	}
	if n := c.InvalidatePrefix("album:"); n != 2 {
		t.Fatalf("invalidated %d, want 2", n)
	}
	if n := c.Len(); n != 2 {
		t.Fatalf("len = %d, want 2", n)
	}
	c.Clear()
	if n := c.Len(); n != 0 {
		t.Fatalf("len after clear = %d", n)
	}
}

func TestGetOrLoad(t *testing.T) {
	c := New[int](0, 0)
	calls := 0
	fetch := func() (int, error) { calls++; return 42, nil }
	for i := 0; i < 3; i++ {
		v, err := c.GetOrLoad("k", fetch)
		if err != nil || v != 42 {
			t.Fatalf("got %v, %v", v, err)
		}
	}
	if calls != 1 {
		t.Fatalf("fetch called %d times, want 1", calls)
	}
}

func TestConcurrentAccess(t *testing.T) {
	c := New[int](128, time.Minute)
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				key := fmt.Sprintf("k:%d", j%64)
				c.Set(key, worker*1000+j)
				c.Get(key)
				c.Get("missing-key") // deterministic miss
				c.InvalidatePrefix("none:")
			}
		}(i)
	}
	wg.Wait()
	if h, m := c.Stats(); h == 0 || m == 0 {
		t.Fatalf("unexpected stats hits=%d misses=%d", h, m)
	}
}
