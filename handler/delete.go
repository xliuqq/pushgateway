// Copyright 2014 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/common/route"

	"github.com/prometheus/pushgateway/storage"
)

// Delete returns a handler that accepts delete requests.
//
// The returned handler is already instrumented for Prometheus.
func Delete(ms storage.MetricStore, jobBase64Encoded bool, logger log.Logger) func(http.ResponseWriter, *http.Request) {
	instrumentedHandler := InstrumentWithCounter(
		"delete",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			job := route.Param(r.Context(), "job")
			if jobBase64Encoded {
				var err error
				if job, err = decodeBase64(job); err != nil {
					http.Error(w, fmt.Sprintf("invalid base64 encoding in job name %q: %v", job, err), http.StatusBadRequest)
					level.Debug(logger).Log("msg", "invalid base64 encoding in job name", "job", job, "err", err.Error())
					return
				}
			}
			labelsString := route.Param(r.Context(), "labels")
			labels, err := splitLabels(labelsString)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				level.Debug(logger).Log("msg", "failed to parse URL", "url", labelsString, "err", err.Error())
				return
			}
			if job == "" {
				http.Error(w, "job name is required", http.StatusBadRequest)
				level.Debug(logger).Log("msg", "job name is required")
				return
			}
			labels["job"] = job
			ms.SubmitWriteRequest(storage.WriteRequest{
				Labels:    labels,
				Timestamp: time.Now(),
			})
			w.WriteHeader(http.StatusAccepted)
		}),
	)

	return func(w http.ResponseWriter, r *http.Request) {
		instrumentedHandler.ServeHTTP(w, r)
	}
}

func getAllLabelsForJob(ms storage.MetricStore, jobName string) []map[string]string {
	familyMaps := ms.GetMetricFamiliesMap()
	var res []map[string]string
	for _, v := range familyMaps {
		currentLabels := v.Labels
		if jobName == currentLabels["job"] {
			res = append(res, currentLabels)
		}
	}
	return res
}

// DeleteJob returns a handler that accepts delete requests for specific job.
//
// The returned handler is already instrumented for Prometheus.
func DeleteJob(ms storage.MetricStore, jobBase64Encoded bool, logger *slog.Logger) func(http.ResponseWriter, *http.Request) {
	instrumentedHandler := InstrumentWithCounter(
		"delete_all",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			job := route.Param(r.Context(), "job")
			if jobBase64Encoded {
				var err error
				if job, err = decodeBase64(job); err != nil {
					http.Error(w, fmt.Sprintf("invalid base64 encoding in job name %q: %v", job, err), http.StatusBadRequest)
					logger.Debug("invalid base64 encoding in job name", "job", job, "err", err.Error())
					return
				}
			}

			if job == "" {
				http.Error(w, "job name is required", http.StatusBadRequest)
				logger.Debug("job name is required")
				return
			}

			var allLabels = getAllLabelsForJob(ms, job)
			for _, labels := range allLabels {
				ms.SubmitWriteRequest(storage.WriteRequest{
					Labels:    labels,
					Timestamp: time.Now(),
				})
			}
			w.WriteHeader(http.StatusAccepted)
		}),
	)

	return func(w http.ResponseWriter, r *http.Request) {
		instrumentedHandler.ServeHTTP(w, r)
	}
}
