package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	inventory_v1 "github.com/shenikar/microservices-course/shared/pkg/proto/inventory/v1"
)

const (
	grpcPort = ":50051" // Порт для InventoryService
)

// server — наша реализация gRPC-сервиса.
type server struct {
	inventory_v1.UnimplementedInventoryServiceServer
	parts map[string]*inventory_v1.Part
}

// newServer создает новый экземпляр сервера с предзаполненными данными.
func newServer() *server {
	s := &server{
		parts: make(map[string]*inventory_v1.Part),
	}
	// Инициализация данных
	initialParts := []*inventory_v1.Part{
		{
			Uuid:          "a1e1b9b0-6d33-424a-8d18-35a8f3d3a4b9",
			Name:          "Звездный двигатель 'Гиперион-9'",
			Description:   "Мощный и надежный двигатель для дальних космических перелетов.",
			Price:         1250000.75,
			StockQuantity: 10,
			Category:      inventory_v1.Category_CATEGORY_ENGINE,
			Dimensions:    &inventory_v1.Dimensions{Length: 15.5, Widht: 5.2, Height: 4.8, Weight: 25000},
			Manufacturer:  &inventory_v1.Manufacturer{Name: "КосмоТех Индастриз", Country: "Марсианская Республика", Website: "https://cosmotech.mars"},
			Tags:          []string{"двигатель", "гипердвигатель", "надежность"},
			CreatedAt:     timestamppb.Now(),
			UpdatedAt:     timestamppb.Now(),
			Metadata: map[string]*inventory_v1.Value{
				"power_rating": {Kind: &inventory_v1.Value_StringValue{"9000 GW"}},
				"warranty":     {Kind: &inventory_v1.Value_Int64Value{12}}, // 12 months
			},
		},
		{
			Uuid:          "c4f2b1a8-3c9b-4b16-8f3a-2d7a8e6c5b4d",
			Name:          "Топливный бак 'Титан'",
			Description:   "Вместительный топливный бак с усиленной защитой от утечек.",
			Price:         350000.00,
			StockQuantity: 50,
			Category:      inventory_v1.Category_CATEGORY_FUEL,
			Dimensions:    &inventory_v1.Dimensions{Length: 20.0, Widht: 10.0, Height: 10.0, Weight: 15000},
			Manufacturer:  &inventory_v1.Manufacturer{Name: "Земные Ресурсные Системы", Country: "Земля", Website: "https://ers.earth"},
			Tags:          []string{"топливо", "бак", "безопасность"},
			CreatedAt:     timestamppb.Now(),
			UpdatedAt:     timestamppb.Now(),
			Metadata: map[string]*inventory_v1.Value{
				"capacity_liters": {Kind: &inventory_v1.Value_DoubleValue{100000.0}},
				"material":        {Kind: &inventory_v1.Value_StringValue{"Titanium-Alloy"}},
			},
		},
	}
	for _, part := range initialParts {
		s.parts[part.Uuid] = part
	}
	return s
}

// GetPart возвращает деталь по её UUID.
func (s *server) GetPart(ctx context.Context, in *inventory_v1.GetPartRequest) (*inventory_v1.GetPartResponse, error) {
	log.Printf("Получен запрос на деталь с UUID: %s", in.GetUuid())
	part, ok := s.parts[in.GetUuid()]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "деталь с UUID %s не найдена", in.GetUuid())
	}
	return &inventory_v1.GetPartResponse{Part: part}, nil
}

// ListParts возвращает список деталей с учетом фильтров.
func (s *server) ListParts(ctx context.Context, in *inventory_v1.ListPartsRequest) (*inventory_v1.ListPartsResponse, error) {
	log.Printf("Получен запрос на список деталей с фильтром: %v", in.GetFilter())

	var result []*inventory_v1.Part
	for _, part := range s.parts {
		result = append(result, part)
	}

	filter := in.GetFilter()
	if filter == nil {
		return &inventory_v1.ListPartsResponse{Parts: result}, nil
	}

	// Фильтрация по UUIDs
	if len(filter.GetUuids()) > 0 {
		filteredResult := []*inventory_v1.Part{}
		uuidSet := make(map[string]struct{})
		for _, uuid := range filter.GetUuids() {
			uuidSet[uuid] = struct{}{}
		}
		for _, part := range result {
			if _, ok := uuidSet[part.Uuid]; ok {
				filteredResult = append(filteredResult, part)
			}
		}
		result = filteredResult
	}

	// Фильтрация по Names
	if len(filter.GetNames()) > 0 {
		filteredResult := []*inventory_v1.Part{}
		nameSet := make(map[string]struct{})
		for _, name := range filter.GetNames() {
			nameSet[name] = struct{}{}
		}
		for _, part := range result {
			if _, ok := nameSet[part.Name]; ok {
				filteredResult = append(filteredResult, part)
			}
		}
		result = filteredResult
	}

	// Фильтрация по Categories
	if len(filter.GetCategories()) > 0 {
		filteredResult := []*inventory_v1.Part{}
		categorySet := make(map[inventory_v1.Category]struct{})
		for _, category := range filter.GetCategories() {
			categorySet[category] = struct{}{}
		}
		for _, part := range result {
			if _, ok := categorySet[part.Category]; ok {
				filteredResult = append(filteredResult, part)
			}
		}
		result = filteredResult
	}

	// Фильтрация по ManufacturerCountries
	if len(filter.GetManufacturerCountries()) > 0 {
		filteredResult := []*inventory_v1.Part{}
		countrySet := make(map[string]struct{})
		for _, country := range filter.GetManufacturerCountries() {
			countrySet[country] = struct{}{}
		}
		for _, part := range result {
			if part.Manufacturer != nil {
				if _, ok := countrySet[part.Manufacturer.Country]; ok {
					filteredResult = append(filteredResult, part)
				}
			}
		}
		result = filteredResult
	}

	// Фильтрация по Tags
	if len(filter.GetTags()) > 0 {
		filteredResult := []*inventory_v1.Part{}
		tagSet := make(map[string]struct{})
		for _, tag := range filter.GetTags() {
			tagSet[tag] = struct{}{}
		}
		for _, part := range result {
			for _, partTag := range part.Tags {
				if _, ok := tagSet[partTag]; ok {
					filteredResult = append(filteredResult, part)
					break
				}
			}
		}
		result = filteredResult
	}

	return &inventory_v1.ListPartsResponse{Parts: result}, nil
}

func main() {
	lis, err := net.Listen("tcp", grpcPort)
	if err != nil {
		log.Fatalf("Ошибка при прослушивании порта %s: %v", grpcPort, err)
	}

	s := grpc.NewServer()
	inventoryServer := newServer()
	inventory_v1.RegisterInventoryServiceServer(s, inventoryServer)
	reflection.Register(s)

	// Graceful shutdown
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Сервер InventoryService запущен на порту %s", grpcPort)
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Ошибка при запуске сервера: %v", err)
		}
	}()

	<-shutdown
	log.Println("Сервер останавливается...")

	s.GracefulStop()

	log.Println("Сервер успешно остановлен.")
}
