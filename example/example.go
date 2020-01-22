package main

import (
	"fmt"

	"github.com/Multi-Tier-Cloud/hash-lookup/hashlookup"
)

func main() {
	testString := "hello-there"
	
	fmt.Println("Looking up:", testString)

	contentHash, _, err := hashlookup.GetHash(testString)
	if err != nil {
		panic(err)
	}

	fmt.Println("Got hash:", contentHash)
}