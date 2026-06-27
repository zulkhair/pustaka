package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"

	"github.com/zulkhair/pustaka/backend/internal/adapter/httpapi"
	"github.com/zulkhair/pustaka/backend/internal/domain"
)

func TestMapDocErrorStatuses(t *testing.T) {
	cases := []struct {
		err  error
		code int
	}{
		{domain.ErrNotFound, http.StatusNotFound},
		{domain.ErrValidation, http.StatusBadRequest},
		{domain.ErrUnsupportedFormat, http.StatusBadRequest},
		{domain.ErrSchemaInvalid, http.StatusBadRequest},
		{domain.ErrForbidden, http.StatusForbidden},
		{assertAnyErr{}, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		app := fiber.New()
		app.Get("/x", func(c *fiber.Ctx) error { return httpapi.MapDocErrorForTest(c, tc.err) })
		req, _ := http.NewRequest(http.MethodGet, "/x", nil)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)
		require.Equal(t, tc.code, resp.StatusCode)
	}
}

type assertAnyErr struct{}

func (assertAnyErr) Error() string { return "boom" }
