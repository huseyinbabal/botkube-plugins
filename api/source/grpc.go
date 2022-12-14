package botkubeplugin

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-plugin"
	botkubeplugin "github.com/huseyinbabal/botkube-plugins/api/source/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"io"
	"log"
)

type SourceGRPCServer struct {
	Impl   Source
	Broker *plugin.GRPCBroker
	botkubeplugin.UnimplementedSourceServer
}

func (p *SourceGRPCServer) Consume(empty *emptypb.Empty, server botkubeplugin.Source_ConsumeServer) error {
	events := make(chan interface{}, 1)
	go p.Impl.Consume(events)
	for {
		select {
		case event := <-events:
			if err := server.Send(&botkubeplugin.ConsumeResponse{Data: fmt.Sprintf("%v", event)}); err != nil {
				return err
			}
		}
	}
}

type SourceGRPCClient struct {
	Broker *plugin.GRPCBroker
	Client botkubeplugin.SourceClient
}

func (p *SourceGRPCClient) Consume(ch chan interface{}) error {
	done := make(chan bool)
	stream, err := p.Client.Consume(context.Background(), &emptypb.Empty{})
	streamContext := stream.Context()
	go func() {
		for {
			response, respErr := stream.Recv()
			if respErr == io.EOF {
				close(done)
				return
			}
			if respErr != nil {
				log.Fatalf("Couldn't receive %v", respErr)
			}
			log.Println(response.Data)
		}
	}()

	go func() {
		<-streamContext.Done()
		if err := streamContext.Err(); err != nil {
			log.Println(err)
		}
		close(done)
	}()
	<-done
	return err
}
