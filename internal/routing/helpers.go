package routing

func NewMono(element any) chan any {
	outChan := make(chan any, 1)
	go func() {
		outChan <- element
		close(outChan)
	}()
	return outChan
}
