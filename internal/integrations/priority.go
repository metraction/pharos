package integrations

func PriorityChannel(high <-chan any, low <-chan any, output chan<- any) {
	for {
		select {
		case msg := <-high: // Handle high priority
			output <- msg
		default:
			select {
			case msg := <-high:
				output <- msg
			case msg := <-low: // Only reached if high is empty
				output <- msg
			}
		}
	}
}
