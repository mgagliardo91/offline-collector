package main

type ChannelStop struct{}

func stopChannel(channel chan chan ChannelStop) {
	stop := make(chan ChannelStop)

	go func() {
		channel <- stop
	}()

	<-stop
}
