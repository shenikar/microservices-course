package main

import (
	"context"
	"log"
	"net"

	"github.com/google/uuid"
	payment_v1 "github.com/shenikar/microservices-course/shared/pkg/proto/payment/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

const (
	grpcPort = ":50052" //Порт для PaymentService
)

type server struct {
	payment_v1.UnimplementedPaymentServiceServer
}

func (s *server) PayOrder(ctx context.Context, in *payment_v1.PayOrderRequest) (*payment_v1.PayOrderResponse, error) {
	transactionUUID := uuid.New().String()
	log.Printf("Оплата заказа [%s] для пользователя [%s] методом [%s] прошла успешно,  transaction_uuid: %s",
		in.GetOrderUuid(), in.GetUserUuid(), in.GetPaymentMethod().String(), transactionUUID)
	return &payment_v1.PayOrderResponse{
		TransactionUuid: transactionUUID,
	}, nil
}

func main() {
	lis, err := net.Listen("tcp", grpcPort)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()

	payment_v1.RegisterPaymentServiceServer(s, &server{})
	reflection.Register(s)

	log.Printf("The server is running on the port %s", grpcPort)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Error when starting the server: %v", err)
	}

}
