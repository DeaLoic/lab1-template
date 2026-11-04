package person

// Person — запись в БД. Опциональные поля — указатели: nil = NULL.
type Person struct {
	ID      int32
	Name    string
	Age     *int32
	Address *string
	Work    *string
}

// Request — тело POST /persons и PATCH /persons/{id}.
// Все поля опциональные: для PATCH отсутствующие поля не изменяются.
type Request struct {
	Name    *string `json:"name"`
	Age     *int32  `json:"age"`
	Address *string `json:"address"`
	Work    *string `json:"work"`
}

// Response — тело ответов API.
type Response struct {
	ID      int32   `json:"id"`
	Name    string  `json:"name"`
	Age     *int32  `json:"age"`
	Address *string `json:"address"`
	Work    *string `json:"work"`
}

// ErrorResponse — тело ответа 404.
type ErrorResponse struct {
	Message string `json:"message"`
}

// ValidationErrorResponse — тело ответа 400.
type ValidationErrorResponse struct {
	Message string            `json:"message"`
	Errors  map[string]string `json:"errors"`
}

func toResponse(p Person) Response {
	return Response{ID: p.ID, Name: p.Name, Age: p.Age, Address: p.Address, Work: p.Work}
}
