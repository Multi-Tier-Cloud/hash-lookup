package main

import (
	"fmt"

	"github.com/Multi-Tier-Cloud/hash-lookup/lookup-client"
)

func main() {
	result, ok := client.GetHash("hello-there")
	if !ok {
		fmt.Println("Error")
	} else {
		fmt.Println("Received:", result)
	}
}