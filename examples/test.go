package main

import "fmt"

func main() {
	test2()
	fmt.Println("Ran test2")
	a := 10
	test1(a)
	fmt.Println("Ran test1 with value: ", a)
	a = test3(a)
	fmt.Println("Ran test3. 'a' is now: ", a)
	test1(a)
}

func test1(a int) {
	fmt.Printf("test1 called with a=%d\n", a)
}

func test2() {
	fmt.Println("test2 called")
}

func test3(a int) int {
	fmt.Println("test3 called")
	return a * 3
}
