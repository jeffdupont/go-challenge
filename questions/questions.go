package main

import "fmt"

func main() {
	names := []string{"john", "tom", "jack"}

	var nameResponse *string
	for _, v := range names {
		if v == "john" {
			nameResponse = &v
		}
	}

	fmt.Printf("%v\n", *nameResponse)
}
