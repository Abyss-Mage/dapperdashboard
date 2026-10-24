package workspace

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"dapper.local/platform/internal/postgres"
)

const maxUploadRequestBytes = 48 << 20

func uploadFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, postgres.ErrUploadInput):
		reply(w, 422, map[string]string{"error": "Check the photos (JPEG, up to 10, 5 MB each) and the post details"})
	case errors.Is(err, postgres.ErrUploadsUnavailable):
		reply(w, 503, map[string]string{"error": "Photo storage is not configured on the server"})
	case errors.Is(err, postgres.ErrBookingNotFound):
		reply(w, 404, map[string]string{"error": "That booking is not linked to your dealership"})
	case errors.Is(err, postgres.ErrUploadNotFound):
		reply(w, 404, map[string]string{"error": "Upload not found"})
	default:
		employeeFailure(w, err)
	}
}

func optionalFloat(value string) (*float64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, true
	}
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, false
	}
	return &f, true
}

// registerEmployeeUploadRoutes serves the employee's own photo posts, hours and breaks. Everything is scoped to
// the signed-in employee; the dealership comes from the administrator's assignment.
func registerEmployeeUploadRoutes(mux *http.ServeMux, store *postgres.Store) {
	mux.HandleFunc("GET /api/v1/employee/hours", func(w http.ResponseWriter, r *http.Request) {
		data, err := store.EmployeeHours(r.Context(), requestSession(r).principalID)
		if err != nil {
			employeeFailure(w, err)
			return
		}
		reply(w, 200, data)
	})
	mux.HandleFunc("GET /api/v1/employee/breaks", func(w http.ResponseWriter, r *http.Request) {
		data, err := store.EmployeeBreaks(r.Context(), requestSession(r).principalID)
		if err != nil {
			employeeFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": data})
	})
	mux.HandleFunc("GET /api/v1/employee/uploads/options", func(w http.ResponseWriter, r *http.Request) {
		items, err := store.EmployeeUploadDealerships(r.Context(), requestSession(r).principalID)
		if err != nil {
			uploadFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"dealerships": items})
	})
	mux.HandleFunc("GET /api/v1/employee/uploads", func(w http.ResponseWriter, r *http.Request) {
		kind := r.URL.Query().Get("kind")
		if kind != "" && kind != "dealership_tag" && kind != "customer_car" {
			reply(w, 400, map[string]string{"error": "Unknown upload kind"})
			return
		}
		items, err := store.ListEmployeeUploads(r.Context(), requestSession(r).principalID, kind, "")
		if err != nil {
			uploadFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("GET /api/v1/employee/uploads/photos/{photo}", func(w http.ResponseWriter, r *http.Request) {
		if !uuid(r.PathValue("photo")) {
			reply(w, 400, map[string]string{"error": "Valid photo required"})
			return
		}
		data, err := store.EmployeeUploadPhoto(r.Context(), requestSession(r).principalID, r.PathValue("photo"))
		if err != nil {
			uploadFailure(w, err)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "private, no-cache")
		w.Header().Set("Content-Disposition", "inline")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})
	mux.HandleFunc("POST /api/v1/employee/uploads", func(w http.ResponseWriter, r *http.Request) {
		// Phones on slow links need longer than the server-wide read window for a multi-photo post.
		controller := http.NewResponseController(w)
		_ = controller.SetReadDeadline(time.Now().Add(3 * time.Minute))
		_ = controller.SetWriteDeadline(time.Now().Add(3 * time.Minute))
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadRequestBytes)
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			reply(w, 400, map[string]string{"error": "Send the post as a multipart form up to 48 MB"})
			return
		}
		defer r.MultipartForm.RemoveAll()
		allowed := map[string]bool{"kind": true, "bookingId": true, "dealershipId": true, "inspectionType": true, "notes": true, "requestKey": true, "latitude": true, "longitude": true, "accuracyMeters": true, "capturedAt": true}
		for key := range r.MultipartForm.Value {
			if !allowed[key] {
				reply(w, 400, map[string]string{"error": "Unsupported field: " + key})
				return
			}
		}
		for key := range r.MultipartForm.File {
			if key != "photos" {
				reply(w, 400, map[string]string{"error": "Unsupported file field"})
				return
			}
		}
		get := func(k string) string {
			if v := r.MultipartForm.Value[k]; len(v) == 1 {
				return v[0]
			}
			return ""
		}
		lat, ok1 := optionalFloat(get("latitude"))
		lon, ok2 := optionalFloat(get("longitude"))
		acc, ok3 := optionalFloat(get("accuracyMeters"))
		captured, err := time.Parse(time.RFC3339Nano, get("capturedAt"))
		if !ok1 || !ok2 || !ok3 || err != nil {
			reply(w, 422, map[string]string{"error": "Check the photos (JPEG, up to 10, 5 MB each) and the post details"})
			return
		}
		files := r.MultipartForm.File["photos"]
		if len(files) < 1 || len(files) > postgres.MaxUploadPhotos {
			reply(w, 422, map[string]string{"error": "Add between 1 and 10 photos"})
			return
		}
		photos := make([][]byte, 0, len(files))
		for _, header := range files {
			if header.Size > postgres.MaxUploadPhotoBytes {
				reply(w, 422, map[string]string{"error": "Each photo must be 5 MB or smaller"})
				return
			}
			f, e := header.Open()
			if e != nil {
				reply(w, 400, map[string]string{"error": "A photo could not be read"})
				return
			}
			data, e := io.ReadAll(io.LimitReader(f, postgres.MaxUploadPhotoBytes+1))
			f.Close()
			if e != nil {
				reply(w, 400, map[string]string{"error": "A photo could not be read"})
				return
			}
			photos = append(photos, data)
		}
		post, err := store.CreateEmployeeUploadPost(r.Context(), requestSession(r).principalID, postgres.UploadPostInput{
			Kind: get("kind"), BookingID: get("bookingId"), DealershipID: get("dealershipId"), InspectionType: get("inspectionType"), Notes: get("notes"), RequestKey: get("requestKey"),
			Latitude: lat, Longitude: lon, Accuracy: acc, CapturedAt: captured, Photos: photos,
		})
		if err != nil {
			uploadFailure(w, err)
			return
		}
		reply(w, 201, post)
	})
}

// registerUploadReviewRoutes lets people with attendance.read at a location review employees' photo posts.
// Every photo viewed (not their own) is written to the append-only view log in the same transaction.
func registerUploadReviewRoutes(mux *http.ServeMux, store *postgres.Store) {
	base := "/api/v1/organizations/{organization}/locations/{location}/employee-uploads"
	resolve := func(w http.ResponseWriter, r *http.Request, action string) (postgres.Scope, bool) {
		org, loc := r.PathValue("organization"), r.PathValue("location")
		if !uuid(org) || !uuid(loc) {
			reply(w, 400, map[string]string{"error": "Valid organization and location selectors required"})
			return postgres.Scope{}, false
		}
		scope, err := store.ResolveScope(r.Context(), requestSession(r).principalID, org, loc, action)
		if err != nil {
			failure(w, err)
			return postgres.Scope{}, false
		}
		return scope, true
	}
	mux.HandleFunc("GET "+base, func(w http.ResponseWriter, r *http.Request) {
		scope, ok := resolve(w, r, "attendance.read")
		if !ok {
			return
		}
		kind := r.URL.Query().Get("kind")
		if kind != "" && kind != "dealership_tag" && kind != "customer_car" {
			reply(w, 400, map[string]string{"error": "Unknown upload kind"})
			return
		}
		items, err := store.ListLocationUploads(r.Context(), scope, kind)
		if err != nil {
			uploadFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("GET "+base+"/photos/{photo}", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := resolve(w, r, "attendance.read")
		if !ok {
			return
		}
		if !uuid(r.PathValue("photo")) {
			reply(w, 400, map[string]string{"error": "Valid photo required"})
			return
		}
		data, err := store.ReviewUploadPhoto(r.Context(), scope, r.PathValue("photo"))
		if err != nil {
			uploadFailure(w, err)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Set("Content-Disposition", "inline")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	})
	mux.HandleFunc("GET "+base+"/views", func(w http.ResponseWriter, r *http.Request) {
		scope, ok := resolve(w, r, "attendance.manage")
		if !ok {
			return
		}
		items, err := store.ListUploadViews(r.Context(), scope)
		if err != nil {
			uploadFailure(w, err)
			return
		}
		reply(w, 200, map[string]any{"items": items})
	})
}
