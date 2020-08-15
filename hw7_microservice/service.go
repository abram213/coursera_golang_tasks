package main

import (
	context "context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"net"
	"strings"
	"sync"
	"time"
)

type ACLs map[string][]string

type MainServer struct {
	acls ACLs
	*BusinessManager
	*AdminManager
}

type BusinessManager struct{}

type AdminManager struct {
	ctx context.Context
	mu  *sync.RWMutex

	loggingBroadcast   chan *Event
	loggingListeners   []chan *Event
	statisticBroadcast chan *statS
	statisticListeners []chan *statS
}

type statS struct {
	consumer string
	method   string
}

func NewStat() Stat {
	return Stat{
		ByConsumer: map[string]uint64{},
		ByMethod:   map[string]uint64{},
	}
}

func NewMainServer(ctx context.Context, acls ACLs) *MainServer {
	var logLs []chan *Event
	var statLs []chan *statS
	logB, statB := make(chan *Event), make(chan *statS)
	return &MainServer{
		acls,
		&BusinessManager{},
		&AdminManager{
			mu:                 &sync.RWMutex{},
			ctx:                ctx,
			loggingBroadcast:   logB,
			loggingListeners:   logLs,
			statisticBroadcast: statB,
			statisticListeners: statLs,
		},
	}
}

func StartMyMicroservice(ctx context.Context, addr, ACLData string) error {
	var acls ACLs
	if err := json.Unmarshal([]byte(ACLData), &acls); err != nil {
		return errors.Wrap(err, "unmarshal ACLs error")
	}

	g, ctx := errgroup.WithContext(ctx)
	ms := NewMainServer(ctx, acls)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	server := grpc.NewServer(
		grpc.UnaryInterceptor(ms.unaryInterceptor),
		grpc.StreamInterceptor(ms.streamInterceptor),
	)

	g.Go(func() error {
		RegisterBizServer(server, ms.BusinessManager)
		RegisterAdminServer(server, ms.AdminManager)
		//fmt.Println("starting server at " + addr)
		return server.Serve(lis)
	})

	g.Go(func() error {
		for {
			select {
			case event := <-ms.loggingBroadcast:
				ms.mu.Lock()
				for _, ch := range ms.loggingListeners {
					ch <- event
				}
				ms.mu.Unlock()
			case signal := <-ms.statisticBroadcast:
				ms.mu.Lock()
				for _, ch := range ms.statisticListeners {
					ch <- signal
				}
				ms.mu.Unlock()
			case <-ctx.Done():
				return nil
			}
		}
	})

	go func() {
		select {
		case <-ctx.Done():
			break
		}
		if server != nil {
			server.GracefulStop()
		}
	}()
	return nil
}

func (bm *BusinessManager) Check(ctx context.Context, in *Nothing) (*Nothing, error) {
	return in, nil
}

func (bm *BusinessManager) Add(ctx context.Context, in *Nothing) (*Nothing, error) {
	return in, nil
}

func (bm *BusinessManager) Test(ctx context.Context, in *Nothing) (*Nothing, error) {
	return in, nil
}

func (am *AdminManager) Logging(in *Nothing, alSrv Admin_LoggingServer) error {
	ch := am.addLogListenersCh()

	for {
		select {
		case event := <-ch:
			if err := alSrv.Send(event); err != nil {
				fmt.Printf("err sending logs from chan %v to client: %v", ch, err)
			}
		case <-am.ctx.Done():
			return nil
		}
	}
}

func (am *AdminManager) Statistics(in *StatInterval, asSrv Admin_StatisticsServer) error {
	ch := am.addStatListenersCh()
	stat := NewStat()

	intervalTicker := time.NewTicker(time.Second * time.Duration(in.IntervalSeconds))
	for {
		select {
		case s := <-ch:
			stat.ByMethod[s.method]++
			stat.ByConsumer[s.consumer]++
		case <-intervalTicker.C:
			if err := asSrv.Send(&stat); err != nil {
				fmt.Printf("err sending stat from %v to client: %v", ch, err)
			}
			stat = NewStat()
		case <-am.ctx.Done():
			return nil
		}
	}
}

func (ms *MainServer) consumerFromCtx(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", errors.New("no metadata in incoming context")
	}
	mdValues := md.Get("consumer")
	if len(mdValues) < 1 {
		return "", status.Errorf(codes.Unauthenticated, "no consumer key in context metadata")
	}
	return mdValues[0], nil
}

func (ms *MainServer) unaryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	consumer, err := ms.consumerFromCtx(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Internal, "can`t get peer from context")
	}
	if err := ms.reqAuth(consumer, info.FullMethod); err != nil {
		return nil, status.Errorf(codes.Unauthenticated, err.Error())
	}

	ms.loggingBroadcast <- &Event{
		Consumer: consumer,
		Method:   info.FullMethod,
		Host:     p.Addr.String(),
	}
	ms.statisticBroadcast <- &statS{
		consumer: consumer,
		method:   info.FullMethod,
	}

	reply, err := handler(ctx, req)

	/*fmt.Printf(`---
	info=%v
	req=%#v
	md=%v
	err=%v
	`,info.FullMethod, req, md, err)*/

	return reply, err
}

func (ms *MainServer) streamInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	consumer, err := ms.consumerFromCtx(ss.Context())
	if err != nil {
		return status.Errorf(codes.Unauthenticated, err.Error())
	}
	p, ok := peer.FromContext(ss.Context())
	if !ok {
		return status.Errorf(codes.Internal, "can`t get peer from context")
	}
	if err := ms.reqAuth(consumer, info.FullMethod); err != nil {
		return status.Errorf(codes.Unauthenticated, err.Error())
	}

	ms.loggingBroadcast <- &Event{
		Consumer: consumer,
		Method:   info.FullMethod,
		Host:     p.Addr.String(),
	}
	ms.statisticBroadcast <- &statS{
		consumer: consumer,
		method:   info.FullMethod,
	}

	/*fmt.Printf(`---
	info=%v
	req=%#v
	err=%v
	`,info.FullMethod, ss.Context(), err)*/

	return handler(srv, ss)
}

func (ms MainServer) reqAuth(consumer, method string) error {
	allowedMethods, ok := ms.acls[consumer]
	if !ok {
		return errors.Errorf("no such consumer: %v in ACL list", consumer)
	}

	for _, m := range allowedMethods {
		allowed := strings.Split(strings.TrimLeft(m, "/"), "/")
		if len(allowed) != 2 {
			continue
		}
		methodParts := strings.Split(strings.TrimLeft(method, "/"), "/")
		if len(methodParts) != 2 {
			continue
		}

		if methodParts[0] == allowed[0] || allowed[0] == "*" {
			if methodParts[1] == allowed[1] || allowed[1] == "*" {
				//access OK!
				return nil
			}
		}
	}
	return errors.Errorf("access denied")
}

func (am *AdminManager) addLogListenersCh() chan *Event {
	am.mu.Lock()
	defer am.mu.Unlock()
	ch := make(chan *Event)
	am.loggingListeners = append(am.loggingListeners, ch)
	return ch
}

func (am *AdminManager) addStatListenersCh() chan *statS {
	am.mu.Lock()
	defer am.mu.Unlock()
	ch := make(chan *statS)
	am.statisticListeners = append(am.statisticListeners, ch)
	return ch
}
