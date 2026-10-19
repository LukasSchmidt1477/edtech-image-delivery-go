package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type publishRequest struct {
	CourseID        string    `json:"course_id"`
	AssetID         string    `json:"asset_id"`
	Image           string    `json:"image"`
	LearnerDeadline time.Time `json:"learner_deadline"`
}

type delivery struct {
	CourseID        string    `json:"course_id"`
	AssetID         string    `json:"asset_id"`
	ObjectKey       string    `json:"object_key"`
	LearnerDeadline time.Time `json:"learner_deadline"`
	Status          string    `json:"status"`
}

func decide(req publishRequest, now time.Time) (delivery, error) {
	if req.CourseID == "" || req.AssetID == "" || req.Image == "" || req.LearnerDeadline.IsZero() {
		return delivery{}, fmt.Errorf("course_id, asset_id, image and learner_deadline are required")
	}
	if !req.LearnerDeadline.After(now) {
		return delivery{}, fmt.Errorf("learner deadline has passed")
	}
	return delivery{req.CourseID, req.AssetID, objectKey(req.CourseID, req.AssetID), req.LearnerDeadline, "published"}, nil
}

type service struct {
	client infraiClient
	bucket string
	now    func() time.Time
	rows   []delivery
}

func (s *service) publish(ctx context.Context, input publishRequest) (delivery, error) {
	row, err := decide(input, s.now())
	if err != nil {
		return delivery{}, err
	}
	data, err := s.client.compress(ctx, input.Image)
	if err != nil {
		return delivery{}, err
	}
	var result struct {
		Image string `json:"image"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return delivery{}, err
	}
	content, err := base64.StdEncoding.DecodeString(result.Image)
	if err != nil || len(content) == 0 {
		return delivery{}, fmt.Errorf("compressed image is empty or invalid")
	}
	url, err := s.client.presign(ctx, s.bucket, row.ObjectKey)
	if err != nil {
		return delivery{}, err
	}
	if err := s.client.put(ctx, url, content); err != nil {
		return delivery{}, err
	}
	s.rows = append(s.rows, row)
	return row, nil
}

func main() {
	key, bucket := os.Getenv("INFRAI_API_KEY"), os.Getenv("INFRAI_BUCKET")
	if key == "" || bucket == "" {
		fmt.Fprintln(os.Stderr, "set INFRAI_API_KEY and INFRAI_BUCKET")
		os.Exit(1)
	}
	client := infraiClient{key: key, http: &http.Client{Timeout: 30 * time.Second}}
	if err := client.bucket(context.Background(), bucket); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	s := &service{client: client, bucket: bucket, now: time.Now}
	http.HandleFunc("POST /courses/assets", func(w http.ResponseWriter, r *http.Request) {
		var input publishRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 12<<20)).Decode(&input); err != nil {
			http.Error(w, "invalid JSON", 400)
			return
		}
		row, err := s.publish(r.Context(), input)
		if err != nil {
			status := http.StatusBadRequest
			if e, ok := err.(*apiError); ok {
				status = e.Status
			} else if err.Error() != "learner deadline has passed" && err.Error() != "course_id, asset_id, image and learner_deadline are required" {
				status = http.StatusBadGateway
			}
			http.Error(w, err.Error(), status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(row)
	})
	http.HandleFunc("GET /educators/report", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"published_count": len(s.rows), "deliveries": s.rows})
	})
	fmt.Println("listening on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
