package model

// ServiceInstance is the normalized service discovery result shared by
// registry adapters. Weight <= 0 means the adapter did not provide a weight.
type ServiceInstance struct {
	URL      string
	Weight   int
	Metadata map[string]string
}
