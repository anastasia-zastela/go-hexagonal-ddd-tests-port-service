package transport

import (
	"context"
	"errors"
	"log"
	"net/http"

	"github.com/anastasia-zastela/port-service/internal/common/server"
	"github.com/anastasia-zastela/port-service/internal/domain"
)

// PortService is a port service
type PortService interface {
	GetPort(ctx context.Context, id string) (*domain.Port, error)
	CountPorts(ctx context.Context) (int, error)
	CreateOrUpdatePort(ctx context.Context, port *domain.Port) error
	// !!!! added later by me
	// CreateOrUpdatePortsBatch(ctx context.Context, port []*domain.Port) error
}

// HttpServer is a HTTP server for ports
type HttpServer struct {
	service PortService
}

// NewHttpServer creates a new HTTP server for ports
func NewHttpServer(service PortService) HttpServer {
	return HttpServer{
		service: service,
	}
}

// CountPorts returns total ports stored in DB
func (h HttpServer) CountPorts(w http.ResponseWriter, r *http.Request) {
	total, err := h.service.CountPorts(r.Context())
	if err != nil {
		server.RespondWithError(err, w, r)
		return
	}

	server.RespondOK(map[string]int{"total": total}, w, r)
}

// GetPort returns a port by ID
func (h HttpServer) GetPort(w http.ResponseWriter, r *http.Request) {
	port, err := h.service.GetPort(r.Context(), r.URL.Query().Get("id"))
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			server.NotFound("port-not-found", err, w, r)
			return
		}
		server.RespondWithError(err, w, r)
		return
	}

	response := Port{
		ID:          port.ID(),
		Name:        port.Name(),
		City:        port.City(),
		Country:     port.Country(),
		Alias:       port.Alias(),
		Regions:     port.Regions(),
		Coordinates: port.Coordinates(),
		Province:    port.Province(),
		Timezone:    port.Timezone(),
		Unlocs:      port.Unlocs(),
	}

	server.RespondOK(response, w, r)
}

// UploadPorts reads ports from JSON file and creates/updates them in DB
func (h HttpServer) UploadPorts(w http.ResponseWriter, r *http.Request) {
	log.Println("uploading ports")

	portChan := make(chan Port)
	errChan := make(chan error)
	doneChan := make(chan struct{})

	go func() {
		err := readPorts(r.Context(), r.Body, portChan)
		if err != nil {
			errChan <- err
		} else {
			doneChan <- struct{}{}
		}
	}()
	portCounter := 0
	for {
		select {
		case <-r.Context().Done():
			log.Printf("request cancelled")
			return
		case <-doneChan:
			log.Printf("finished reading ports")
			server.RespondOK(map[string]int{"total_ports": portCounter}, w, r)
			return
		case err := <-errChan:
			log.Printf("error while parsing port json: %+v", err)
			server.BadRequest("invalid-json", err, w, r)
			return
		case port := <-portChan:
			portCounter++
			log.Printf("[%d] received port: %+v", portCounter, port)
			p, err := portHttpToDomain(&port)
			if err != nil {
				server.BadRequest("port-to-domain", err, w, r)
				return
			}
			if err := h.service.CreateOrUpdatePort(r.Context(), p); err != nil {
				server.RespondWithError(err, w, r)
				return
			}
		}
	}
}

// !!!! added later by me
// func (h HttpServer) UploadPortsBatch(w http.ResponseWriter, r *http.Request) {
// 	log.Println("uploading ports")

// 	portChan := make(chan Port)
// 	errChan := make(chan error, 1) // buffered to allow sending an error without blocking
// 	doneChan := make(chan struct{})

// 	// Start a goroutine to read ports and send them to portChan
// 	go func() {
// 		defer close(portChan)
// 		if err := readPorts(r.Context(), r.Body, portChan); err != nil {
// 			errChan <- err
// 		} else {
// 			doneChan <- struct{}{}
// 		}
// 	}()

// 	// Start a batch processing goroutine
// 	batch := make([]*domain.Port, 0, 100) // Adjust batch size as needed
// 	batchProcessingDone := make(chan error, 1)

// 	go func() {
// 		defer close(batchProcessingDone)
// 		for port := range portChan {
// 			domainPort, err := portHttpToDomain(&port)
//             if err != nil {
//                 batchProcessingDone <- fmt.Errorf("error converting port to domain: %w", err)
//                 return
//             }
// 			batch = append(batch, domainPort)
// 			if len(batch) >= cap(batch) {
// 				if err := h.service.CreateOrUpdatePortsBatch(r.Context(), batch); err != nil {
// 					batchProcessingDone <- err
// 					return
// 				}
// 				batch = batch[:0] // reset the batch slice without additional memory allocation
// 			}
// 		}
// 		// process remaining ports in the batch
// 		if len(batch) > 0 {
// 			if err := h.service.CreateOrUpdatePortsBatch(r.Context(), batch); err != nil {
// 				batchProcessingDone <- err
// 				return
// 			}
// 		}
// 		batchProcessingDone <- nil // indicate successful completion
// 	}()

// 	select {
// 	case <-r.Context().Done():
// 		log.Println("request cancelled")
// 		server.RespondWithError(fmt.Errorf("request cancelled"), w, r)
// 		return
// 	case err := <-errChan:
// 		log.Printf("error while parsing port json: %+v", err)
// 		server.BadRequest("invalid-json", err, w, r)
// 		return
// 	case err := <-batchProcessingDone:
// 		if err != nil {
// 			log.Printf("error while processing batch: %+v", err)
// 			server.RespondWithError(err, w, r)
// 			return
// 		}
// 		log.Printf("finished reading and processing ports")
// 		server.RespondOK(map[string]int{"status": "completed"}, w, r)
// 	}
// }
