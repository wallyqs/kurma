// Copyright 2015-2016 Apcera Inc. All rights reserved.

package apiproxy

import (
	"encoding/json"
	"net/http"
)

func (s *Server) infoRequest(w http.ResponseWriter, req *http.Request) {
	hostInfo, err := s.client.Info()
	if err != nil {
		s.log.Errorf("Failed to get host info: %v", err)
		http.Error(w, "Failed to process request", 500)
		return
	}
	json.NewEncoder(w).Encode(hostInfo)
}
