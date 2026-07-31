package service

import "Qavor/internal/model/dto/response"

type ProcessingJobService interface {
	Get(jobID string) (*response.DocumentProcessingJobResponse, error)
	List(limit int) (*response.DocumentProcessingJobListResponse, error)
	Retry(jobID string) (*response.DocumentProcessingJobResponse, error)
}
