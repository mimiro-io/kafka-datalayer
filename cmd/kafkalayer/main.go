package main

import (
	kafkalayer "github.com/mimiro.io/kafka-datalayer/kafka-datalayer/internal/app"
)

func main() {
	kafkalayer.Wire().Run()
}
