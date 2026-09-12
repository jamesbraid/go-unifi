package unifi

import (
	"context"
	"net/http"
)

// This file holds the request and response handling the generated resource
// clients share. Two response contracts cover every resource the controller
// exposes: the v1 REST endpoints wrap everything in {"meta":…,"data":[…]},
// answering a single object as a one-element array, while the v2 endpoints
// answer the value itself.

// envelope is the v1 response body. Meta carries the controller's own status,
// which the readers below ignore: a request that failed has already been
// turned into an error by do.
type envelope[T any] struct {
	Meta meta `json:"meta"`
	Data []T  `json:"data"`
}

// onlyElement returns the sole element of a collection, and NotFoundError when
// the collection does not hold exactly one. Reading one v1 object means asking
// for a collection and insisting it came back with one thing in it.
func onlyElement[T any](items []T) (*T, error) {
	if len(items) != 1 {
		return nil, &NotFoundError{}
	}
	return &items[0], nil
}

// envelopeList reads a v1 collection.
func envelopeList[T any](
	ctx context.Context,
	c *ApiClient,
	path string,
	query ...map[string]string,
) ([]T, error) {
	var resp envelope[T]
	if err := c.do(ctx, http.MethodGet, path, nil, &resp, query...); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// envelopeOne issues a v1 request that answers with exactly one object: a read
// by id, or a create.
func envelopeOne[T any](ctx context.Context, c *ApiClient, method, path string, body any) (*T, error) {
	var resp envelope[T]
	if err := c.do(ctx, method, path, body, &resp); err != nil {
		return nil, err
	}
	return onlyElement(resp.Data)
}

// envelopeUpdate issues a v1 PUT.
//
// A successful write does not always answer with the object it wrote: the UDM
// SE returns an empty data array. So an empty answer is re-read through reread
// rather than reported as NotFound.
func envelopeUpdate[T any](
	ctx context.Context,
	c *ApiClient,
	path string,
	body any,
	reread func() (*T, error),
) (*T, error) {
	var resp envelope[T]
	if err := c.do(ctx, http.MethodPut, path, body, &resp); err != nil {
		return nil, err
	}
	if len(resp.Data) == 0 {
		return reread()
	}
	return onlyElement(resp.Data)
}

// envelopeMasked writes only the named wire fields to a v1 endpoint. Resolving
// the mask is a failure of its own -- an unknown field name is an error, not a
// silent omission -- so it happens before anything reaches the wire. See
// maskedBody.
func envelopeMasked[T any](
	ctx context.Context,
	c *ApiClient,
	path string,
	d *T,
	fields []string,
	reread func() (*T, error),
) (*T, error) {
	body, err := maskedBody(d, fields)
	if err != nil {
		return nil, err
	}
	return envelopeUpdate(ctx, c, path, body, reread)
}

// bareList reads a v2 collection, whose body is the array itself.
func bareList[T any](
	ctx context.Context,
	c *ApiClient,
	path string,
	query ...map[string]string,
) ([]T, error) {
	var resp []T
	if err := c.do(ctx, http.MethodGet, path, nil, &resp, query...); err != nil {
		return nil, err
	}
	return resp, nil
}

// bareOne issues a v2 request whose body is the object itself.
func bareOne[T any](ctx context.Context, c *ApiClient, method, path string, body any) (*T, error) {
	var resp T
	if err := c.do(ctx, method, path, body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// bareMasked writes only the named wire fields to a v2 endpoint. See
// envelopeMasked for why the mask resolves first.
func bareMasked[T any](ctx context.Context, c *ApiClient, path string, d *T, fields []string) (*T, error) {
	body, err := maskedBody(d, fields)
	if err != nil {
		return nil, err
	}
	return bareOne[T](ctx, c, http.MethodPut, path, body)
}

// findByID picks one v2 object out of a collection.
//
// The v2 resources publish no per-id GET -- that path answers HTTP 405 -- so
// reading one of them means listing them all and picking the match. idOf names
// the field carrying the id, which generics cannot reach on their own.
func findByID[T any](items []T, id string, idOf func(*T) string) (*T, error) {
	for i := range items {
		if idOf(&items[i]) == id {
			return &items[i], nil
		}
	}
	return nil, &NotFoundError{}
}

// deleteResource deletes one object. The empty object is the request body the
// controller expects; nothing in the response is worth decoding.
func deleteResource(ctx context.Context, c *ApiClient, path string) error {
	return c.do(ctx, http.MethodDelete, path, struct{}{}, nil)
}
