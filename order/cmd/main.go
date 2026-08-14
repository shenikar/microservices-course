package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	inventory_v1 "github.com/shenikar/microservices-course/shared/pkg/proto/inventory/v1"
	payment_v1 "github.com/shenikar/microservices-course/shared/pkg/proto/payment/v1"

	oapi "github.com/shenikar/microservices-course/shared/pkg/openapi/order/v1"
)

const (
	httpPort          = ":8080"
	inventoryGRPCAddr = "localhost:50051"
	paymentGRPCAddr   = "localhost:50052"
)

// OrderStatus представляет статус заказа.
type OrderStatus string

const (
	OrderStatusPendingPayment OrderStatus = "PENDING_PAYMENT"
	OrderStatusPaid           OrderStatus = "PAID"
	OrderStatusCancelled      OrderStatus = "CANCELLED"
)

// PaymentMethod представляет способ оплаты.
type PaymentMethod string

const (
	PaymentMethodCard          PaymentMethod = "CARD"
	PaymentMethodSbp           PaymentMethod = "SBP"
	PaymentMethodCreditCard    PaymentMethod = "CREDIT_CARD"
	PaymentMethodInvestorMoney PaymentMethod = "INVESTOR_MONEY"
)

// Order представляет структуру заказа.
type Order struct {
	UUID            string
	UserUUID        string
	PartUUIDs       []string
	TotalPrice      float64
	TransactionUUID string
	PaymentMethod   PaymentMethod
	Status          OrderStatus
}

// OrderStorage представляет потокобезопасное хранилище заказов.
type OrderStorage struct {
	mu     sync.RWMutex
	orders map[string]*Order
}

// NewOrderStorage создает новое хранилище заказов.
func NewOrderStorage() *OrderStorage {
	return &OrderStorage{
		orders: make(map[string]*Order),
	}
}

// Get возвращает заказ по UUID.
func (s *OrderStorage) Get(uuid string) (*Order, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	order, ok := s.orders[uuid]
	return order, ok
}

// Save сохраняет или обновляет заказ.
func (s *OrderStorage) Save(order *Order) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.orders[order.UUID] = order
}

// OrderServiceHandler реализует интерфейс, сгенерированный ogen.
type OrderServiceHandler struct {
	oapi.UnimplementedHandler
	storage         *OrderStorage
	inventoryClient inventory_v1.InventoryServiceClient
	paymentClient   payment_v1.PaymentServiceClient
}

// NewOrderServiceHandler создает новый экземпляр OrderServiceHandler.
func NewOrderServiceHandler(
	storage *OrderStorage,
	invClient inventory_v1.InventoryServiceClient,
	payClient payment_v1.PaymentServiceClient,
) *OrderServiceHandler {
	return &OrderServiceHandler{
		storage:         storage,
		inventoryClient: invClient,
		paymentClient:   payClient,
	}
}

// CreateOrder implements createOrder operation.
func (h *OrderServiceHandler) CreateOrder(ctx context.Context, req *oapi.CreateOrderRequest) (oapi.CreateOrderRes, error) {
	// 1. Получить детали запчастей из InventoryService и проверить их наличие.
	// 2. Рассчитать общую стоимость.
	// 3. Создать новый заказ со статусом PENDING_PAYMENT.
	// 4. Сохранить заказ в хранилище.
	// 5. Вернуть order_uuid и total_price.

	// Конвертируем []uuid.UUID в []string для gRPC-запроса
	partUUIDsAsStrings := make([]string, len(req.PartUuids))
	for i, u := range req.PartUuids {
		partUUIDsAsStrings[i] = u.String()
	}

	// Логика получения деталей из InventoryService
	partsResp, err := h.inventoryClient.ListParts(ctx, &inventory_v1.ListPartsRequest{
		Filter: &inventory_v1.PartsFilter{
			Uuids: partUUIDsAsStrings,
		},
	})
	if err != nil {
		log.Printf("Ошибка при получении деталей из InventoryService: %v", err)
		return &oapi.InternalServerError{Code: oapi.NewOptInt32(500), Message: oapi.NewOptString("Internal server error")}, nil
	}

	if len(partsResp.Parts) != len(req.PartUuids) {
		// Некоторые детали не найдены
		return &oapi.BadRequestError{Code: oapi.NewOptInt32(400), Message: oapi.NewOptString("One or more parts not found")}, nil
	}

	totalPrice := 0.0
	for _, part := range partsResp.Parts {
		totalPrice += part.Price
	}

	orderUUID := uuid.New().String()
	order := &Order{
		UUID:       orderUUID,
		UserUUID:   req.UserUUID.String(),
		PartUUIDs:  partUUIDsAsStrings, // Используем сконвертированный слайс
		TotalPrice: totalPrice,
		Status:     OrderStatusPendingPayment,
	}
	h.storage.Save(order)

	log.Printf("Заказ %s создан для пользователя %s", orderUUID, req.UserUUID)

	return &oapi.CreateOrderResponse{
		OrderUUID:  oapi.NewOptUUID(uuid.MustParse(orderUUID)),
		TotalPrice: oapi.NewOptFloat64(totalPrice),
	}, nil
}

// GetOrder implements getOrder operation.
func (h *OrderServiceHandler) GetOrder(ctx context.Context, params oapi.GetOrderParams) (oapi.GetOrderRes, error) {
	// 1. Найти заказ по order_uuid.
	// 2. Если не найден - вернуть 404.
	// 3. Вернуть информацию о заказе.
	orderUUID := params.OrderUUID.String()
	order, ok := h.storage.Get(orderUUID)
	if !ok {
		return &oapi.NotFoundError{Code: oapi.NewOptInt32(404), Message: oapi.NewOptString(fmt.Sprintf("Order %s not found", orderUUID))}, nil
	}

	var transactionUUID uuid.UUID
	if order.TransactionUUID != "" {
		transactionUUID = uuid.MustParse(order.TransactionUUID)
	}

	// Конвертируем []string в []uuid.UUID для ответа
	partUUIDsAsUUIDs := make([]uuid.UUID, len(order.PartUUIDs))
	for i, s := range order.PartUUIDs {
		partUUIDsAsUUIDs[i] = uuid.MustParse(s)
	}

	return &oapi.GetOrderResponse{
		OrderUUID:       oapi.NewOptUUID(uuid.MustParse(order.UUID)),
		UserUUID:        oapi.NewOptUUID(uuid.MustParse(order.UserUUID)),
		PartUuids:       partUUIDsAsUUIDs,
		TotalPrice:      oapi.NewOptFloat64(order.TotalPrice),
		TransactionUUID: oapi.NewOptUUID(transactionUUID),
		PaymentMethod:   oapi.OptPaymentMethod{Value: oapi.PaymentMethod(order.PaymentMethod), Set: order.PaymentMethod != ""},
		Status:          oapi.NewOptOrderStatus(oapi.OrderStatus(order.Status)),
	}, nil
}

// PayOrder implements payOrder operation.
func (h *OrderServiceHandler) PayOrder(ctx context.Context, req *oapi.PayOrderRequest, params oapi.PayOrderParams) (oapi.PayOrderRes, error) {
	// 1. Найти заказ по order_uuid. Если не найден - вернуть 404.
	// 2. Вызвать PaymentService.PayOrder.
	// 3. Обновить статус заказа и сохранить transaction_uuid.
	orderUUID := params.OrderUUID.String()
	order, ok := h.storage.Get(orderUUID)
	if !ok {
		return &oapi.NotFoundError{Code: oapi.NewOptInt32(404), Message: oapi.NewOptString(fmt.Sprintf("Order %s not found", orderUUID))}, nil
	}

	paymentMethod := payment_v1.PaymentMethod(payment_v1.PaymentMethod_value[string(req.PaymentMethod)])

	payResp, err := h.paymentClient.PayOrder(ctx, &payment_v1.PayOrderRequest{
		OrderUuid:     order.UUID,
		UserUuid:      order.UserUUID,
		PaymentMethod: paymentMethod,
	})
	if err != nil {
		log.Printf("Ошибка при оплате заказа через PaymentService: %v", err)
		// ВАЖНО: Контракт для PayOrder не определяет InternalServerError.
		// Чтобы код компилировался, возвращаем NotFoundError, хотя семантически это неверно.
		// В реальном проекте следовало бы исправить OpenAPI контракт.
		return &oapi.NotFoundError{Code: oapi.NewOptInt32(404), Message: oapi.NewOptString("Could not process payment")}, nil
	}

	order.TransactionUUID = payResp.TransactionUuid
	order.PaymentMethod = PaymentMethod(req.PaymentMethod)
	order.Status = OrderStatusPaid
	h.storage.Save(order)

	log.Printf("Заказ %s оплачен, transaction_uuid: %s", order.UUID, order.TransactionUUID)

	return &oapi.PayOrderResponse{
		TransactionUUID: oapi.NewOptUUID(uuid.MustParse(order.TransactionUUID)),
	}, nil
}

// CancelOrder implements cancelOrder operation.
func (h *OrderServiceHandler) CancelOrder(ctx context.Context, params oapi.CancelOrderParams) (oapi.CancelOrderRes, error) {
	// 1. Найти заказ по order_uuid.
	// 2. Если не найден - вернуть 404.
	// 3. Проверить статус. Если PAID - вернуть 409.
	// 4. Изменить статус на CANCELLED.
	orderUUID := params.OrderUUID.String()
	order, ok := h.storage.Get(orderUUID)
	if !ok {
		return &oapi.NotFoundError{Code: oapi.NewOptInt32(404), Message: oapi.NewOptString(fmt.Sprintf("Order %s not found", orderUUID))}, nil
	}

	if order.Status == OrderStatusPaid {
		return &oapi.ConflictError{Code: oapi.NewOptInt32(409), Message: oapi.NewOptString(fmt.Sprintf("Order %s is already paid and cannot be cancelled", orderUUID))}, nil
	}

	order.Status = OrderStatusCancelled
	h.storage.Save(order)

	log.Printf("Заказ %s отменен", order.UUID)

	return &oapi.CancelOrderNoContent{}, nil // 204 No Content
}

func main() {
	// Инициализация gRPC клиентов
	invConn, err := grpc.NewClient(inventoryGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Ошибка при подключении к InventoryService: %v", err)
	}
	defer invConn.Close()
	inventoryClient := inventory_v1.NewInventoryServiceClient(invConn)

	payConn, err := grpc.NewClient(paymentGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Ошибка при подключении к PaymentService: %v", err)
	}
	defer payConn.Close()
	paymentClient := payment_v1.NewPaymentServiceClient(payConn)

	// Инициализация хранилища заказов и обработчика HTTP запросов
	orderStorage := NewOrderStorage()
	orderHandler := NewOrderServiceHandler(orderStorage, inventoryClient, paymentClient)

	// Создание маршрутизатора HTTP
	server, err := oapi.NewServer(orderHandler)
	if err != nil {
		log.Fatalf("Ошибка при создании Ogen server: %v", err)
	}

	log.Printf("HTTP OrderService запущен на порту %s", httpPort)

	log.Fatal(http.ListenAndServe(httpPort, server))
}
