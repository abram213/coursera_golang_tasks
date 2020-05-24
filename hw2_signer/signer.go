package main

import (
	"sort"
	"strconv"
	"strings"
	"sync"
)

func worker(in, out chan interface{}, job job, wg *sync.WaitGroup) {
	defer wg.Done()
	defer close(out)
	job(in, out)
}

func ExecutePipeline(jobs ...job) {
	in := make(chan interface{}, 10)
	wg := &sync.WaitGroup{}
	for _, job := range jobs {
		out := make(chan interface{}, 10)
		wg.Add(1)
		go worker(in, out, job, wg)
		in = out
	}
	wg.Wait()
}

func SingleHash(in, out chan interface{}) {
	maxMd5Func := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}
	for input := range in {
		value, ok := input.(int)
		if !ok {
			panic("type accession error")
		}
		wg.Add(1)
		go func(input string) {
			defer wg.Done()
			a := asyncFunc(DataSignerCrc32, input)
			md5 := asyncFuncWithLimit(DataSignerMd5, input, maxMd5Func)
			b := asyncFunc(DataSignerCrc32, <- md5)
			out <- <-a + "~" + <-b
		}(strconv.Itoa(value))
	}
	wg.Wait()
}

func MultiHash(in, out chan interface{}) {
	wg := &sync.WaitGroup{}
	mu := &sync.Mutex{}
	ths := []int{0,1,2,3,4,5}
	for input := range in {
		value, ok := input.(string)
		if !ok {
			panic("type accession error")
		}
		wg.Add(1)
		go func(input string) {
			defer wg.Done()
			waitStr := &sync.WaitGroup{}
			hashes := map[int]string{}
			for _, th := range ths {
				waitStr.Add(1)
				go func(n int) {
					defer waitStr.Done()
					res := <-asyncFunc(DataSignerCrc32, strconv.Itoa(n) + input)
					mu.Lock()
					hashes[n] = res
					mu.Unlock()
				}(th)
			}
			waitStr.Wait()
			result := ""
			for _, th := range ths {
				mu.Lock()
				result += hashes[th]
				mu.Unlock()
			}
			out <- result
		}(value)
	}
	wg.Wait()
}

func CombineResults(in, out chan interface{}) {
	var str []string
	for input := range in {
		str = append(str, input.(string))
	}
	sort.Strings(str)
	result := strings.Join(str, "_")
	out <- result
}

func asyncFuncWithLimit(f func(data string) string, data string, limitCh chan struct{}) chan string {
	result := make(chan string, 1)
	go func(out chan<- string) {
		limitCh <- struct{}{}
		out <- f(data)
		<- limitCh
	}(result)
	return result
}

func asyncFunc(f func(data string) string, data string) chan string {
	result := make(chan string, 1)
	go func(out chan<- string) {
		out <- f(data)
	}(result)
	return result
}
