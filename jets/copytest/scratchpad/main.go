package main

import (
	"fmt"
	"regexp"
)

func main() {
	expr := "^\\d{1,5}\\s(M[^CG](\\w*)|[^M\\[{#}](\\w*))"

	r, err := regexp.Compile(expr)
	fmt.Println(err)
	fmt.Println(r.MatchString("123 #123]"))

	// match, _ := regexp.MatchString("^\\d{1,5}\\s\\w*", "123 [123]")
	// fmt.Println(match)

}
