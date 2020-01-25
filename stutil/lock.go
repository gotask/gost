package stutil

import (
	"sync"
)

func LockWaitGroup(num int, f func(int) int) []int {
	var wg sync.WaitGroup
	r := make([]int, num, num)
	for i := 0; i < num; i++ {
		wg.Add(1)
		go func(index int) {
			r[index] = f(index)
			wg.Done()
		}(i)
	}
	wg.Wait()
	return r
}
