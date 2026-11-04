package person

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// fakeRepo — in-memory реализация Repository для unit-тестов.
type fakeRepo struct {
	next  int32
	items map[int32]Person
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{next: 1, items: make(map[int32]Person)}
}

func (f *fakeRepo) Create(_ context.Context, p Person) (int32, error) {
	p.ID = f.next
	f.next++
	f.items[p.ID] = p
	return p.ID, nil
}

func (f *fakeRepo) GetByID(_ context.Context, id int32) (Person, error) {
	p, ok := f.items[id]
	if !ok {
		return Person{}, ErrNotFound
	}
	return p, nil
}

func (f *fakeRepo) List(_ context.Context) ([]Person, error) {
	out := make([]Person, 0, len(f.items))
	for i := int32(1); i < f.next; i++ {
		if p, ok := f.items[i]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeRepo) Update(_ context.Context, id int32, p Person) (Person, error) {
	if _, ok := f.items[id]; !ok {
		return Person{}, ErrNotFound
	}
	p.ID = id
	f.items[id] = p
	return p, nil
}

func (f *fakeRepo) Delete(_ context.Context, id int32) error {
	if _, ok := f.items[id]; !ok {
		return ErrNotFound
	}
	delete(f.items, id)
	return nil
}

func (f *fakeRepo) Close() {}

func ptr[T any](v T) *T { return &v }

// newTestServer поднимает HTTP-обработчики над fake-хранилищем.
func newTestServer() *httptest.Server {
	mux := http.NewServeMux()
	NewHandler(NewService(newFakeRepo())).Register(mux)
	return httptest.NewServer(mux)
}

func doJSON(t *testing.T, srv *httptest.Server, method, path string, body any) *http.Response {
	t.Helper()
	var payload io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		payload = bytes.NewReader(buf)
	}
	req, err := http.NewRequest(method, srv.URL+path, payload)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

// TestCreatePerson проверяет POST /persons: 201, пустое тело и Location.
func TestCreatePerson(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	t.Run("success", func(t *testing.T) {
		req := Request{
			Name:    ptr("Ivan"),
			Age:     ptr(int32(31)),
			Address: ptr("Tverskaya 1"),
			Work:    ptr("Acme"),
		}
		resp := doJSON(t, srv, http.MethodPost, "/api/v1/persons", req)

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("status = %d, want 201", resp.StatusCode)
		}
		if loc := resp.Header.Get("Location"); loc != "/api/v1/persons/1" {
			t.Errorf("Location = %q, want /api/v1/persons/1", loc)
		}
		body, _ := io.ReadAll(resp.Body)
		if len(bytes.TrimSpace(body)) != 0 {
			t.Errorf("expected empty body, got %q", body)
		}
	})

	t.Run("validation error", func(t *testing.T) {
		resp := doJSON(t, srv, http.MethodPost, "/api/v1/persons", Request{Name: ptr("  ")})

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		var got ValidationErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if _, ok := got.Errors["name"]; !ok {
			t.Errorf("expected error for field name, got %v", got.Errors)
		}
	})
}

// TestGetPerson проверяет GET /persons/{id}: 200 с полями и 404 для отсутствующего.
func TestGetPerson(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	created := doJSON(t, srv, http.MethodPost, "/api/v1/persons", Request{
		Name: ptr("Maria"), Age: ptr(int32(25)), Address: ptr("Arbat 2"), Work: ptr("Globex"),
	})
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want 201", created.StatusCode)
	}

	t.Run("found", func(t *testing.T) {
		resp := doJSON(t, srv, http.MethodGet, "/api/v1/persons/1", nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		want := Response{ID: 1, Name: "Maria", Age: ptr(int32(25)), Address: ptr("Arbat 2"), Work: ptr("Globex")}
		var got Response
		if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("not found", func(t *testing.T) {
		resp := doJSON(t, srv, http.MethodGet, "/api/v1/persons/999", nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", resp.StatusCode)
		}
		var got ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got.Message == "" {
			t.Error("expected non-empty message")
		}
	})
}

// TestListPersons проверяет GET /persons: массив, [] для пустого хранилища.
func TestListPersons(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	t.Run("empty", func(t *testing.T) {
		resp := doJSON(t, srv, http.MethodGet, "/api/v1/persons", nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		var got []Response
		if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("expected empty array, got %d items", len(got))
		}
	})

	t.Run("two persons", func(t *testing.T) {
		doJSON(t, srv, http.MethodPost, "/api/v1/persons", Request{Name: ptr("A")})
		doJSON(t, srv, http.MethodPost, "/api/v1/persons", Request{Name: ptr("B")})

		resp := doJSON(t, srv, http.MethodGet, "/api/v1/persons", nil)
		var got []Response
		if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 persons, got %d", len(got))
		}
		if got[0].Name != "A" || got[1].Name != "B" {
			t.Errorf("unexpected order: %+v", got)
		}
	})
}

// TestPatchPerson проверяет PATCH: частичное обновление сохраняет остальные поля.
func TestPatchPerson(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	doJSON(t, srv, http.MethodPost, "/api/v1/persons", Request{
		Name: ptr("Ivan"), Age: ptr(int32(31)), Address: ptr("Old st."), Work: ptr("Acme"),
	})

	t.Run("merges partial fields", func(t *testing.T) {
		resp := doJSON(t, srv, http.MethodPatch, "/api/v1/persons/1", Request{
			Name: ptr("Vanya"), Address: ptr("New st."),
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		want := Response{ID: 1, Name: "Vanya", Age: ptr(int32(31)), Address: ptr("New st."), Work: ptr("Acme")}
		var got Response
		if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("not found", func(t *testing.T) {
		resp := doJSON(t, srv, http.MethodPatch, "/api/v1/persons/999", Request{Name: ptr("X")})
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", resp.StatusCode)
		}
	})
}

// TestDeletePerson проверяет DELETE: 204 и последующий 404.
func TestDeletePerson(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	doJSON(t, srv, http.MethodPost, "/api/v1/persons", Request{Name: ptr("Ivan")})

	t.Run("existing", func(t *testing.T) {
		resp := doJSON(t, srv, http.MethodDelete, "/api/v1/persons/1", nil)
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("status = %d, want 204", resp.StatusCode)
		}

		check := doJSON(t, srv, http.MethodGet, "/api/v1/persons/1", nil)
		if check.StatusCode != http.StatusNotFound {
			t.Errorf("status after delete = %d, want 404", check.StatusCode)
		}
	})

	t.Run("not found", func(t *testing.T) {
		resp := doJSON(t, srv, http.MethodDelete, "/api/v1/persons/999", nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", resp.StatusCode)
		}
	})
}
