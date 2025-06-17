package routing

func NewMono(element any) chan any {
	outChan := make(chan any)
	go func() {
		outChan <- element
		close(outChan)
	}()
	return outChan
}
