package chromedp

func runListeners(list []cancelableListener, ev interface{}) []cancelableListener {
	for i := 0; i < len(list); {
		listener := list[i]
		select {
		case <-listener.ctx.Done():
			list = append(list[:i], list[i+1:]...)
			continue
		default:
			listener.fn(ev)
			i++
		}
	}
	return list
}
