package main

import (
	"context"
	"log"

	"github.com/google/uuid"
	payment_v1 "github.com/shenikar/microservices-course/shared/pkg/proto/payment/v1"
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

}
