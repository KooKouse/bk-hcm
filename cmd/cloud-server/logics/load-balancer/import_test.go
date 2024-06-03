package lblogic

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestListenerRawInput_validateRS(t *testing.T) {
	cases := []struct {
		input     *ListenerRawInput
		expectErr bool
	}{
		{
			input: &ListenerRawInput{
				RSIPs:   []string{"192.168.1.3"},
				RSPorts: []int{80},
				Weight:  []int{1},
			},
			expectErr: false,
		},
		{
			input: &ListenerRawInput{
				RSIPs:   []string{"192.168.1.3"},
				RSPorts: []int{80, 8081},
				Weight:  []int{1, 2},
			},
			expectErr: true,
		},
		{
			input: &ListenerRawInput{
				RSIPs:   []string{"192.168.1.3"},
				RSPorts: []int{80, 8081},
				Weight:  []int{1},
			},
			expectErr: true,
		},
		{
			input: &ListenerRawInput{
				RSIPs:   []string{"192.168.1.3", "192.168.1.4"},
				RSPorts: []int{80},
				Weight:  []int{1},
			},
			expectErr: false,
		},
		{
			input: &ListenerRawInput{
				RSIPs:   []string{"192.168.1.3", "192.168.1.4"},
				RSPorts: []int{80, 8081},
				Weight:  []int{1, 2},
			},
			expectErr: false,
		},
		{
			input: &ListenerRawInput{
				RSIPs:   []string{"192.168.1.3", "192.168.1.4"},
				RSPorts: []int{80},
				Weight:  []int{1, 2},
			},
			expectErr: false,
		},
		{
			input: &ListenerRawInput{
				RSIPs:   []string{"192.168.1.3", "192.168.1.4"},
				RSPorts: []int{80},
				Weight:  []int{1, 2, 3},
			},
			expectErr: true,
		},
	}

	for i, c := range cases {
		err := c.input.validateRS()
		if c.expectErr {
			assert.Error(t, err, "case %d failed", i)
		} else {
			assert.NoError(t, err, "case %d failed", i)
		}
	}
}

func TestListenerRawInput_validateProtocol(t *testing.T) {
	cases := []struct {
		input     *ListenerRawInput
		expectErr bool
	}{
		{
			input: &ListenerRawInput{
				Protocol: "TCP",
			},
			expectErr: false,
		},
		{
			input: &ListenerRawInput{
				Protocol: "UDP",
				Domain:   "example.com",
			},
			expectErr: true,
		},
		{
			input: &ListenerRawInput{
				Protocol: "TCP",
				URLPath:  "/api",
			},
			expectErr: true,
		},
		{
			input: &ListenerRawInput{
				Protocol:   "TCP",
				ServerCert: "server.crt",
			},
			expectErr: true,
		},
		{
			input: &ListenerRawInput{
				Protocol:   "HTTPS",
				Domain:     "example.com",
				ServerCert: "server.crt",
				URLPath:    "/api",
			},
			expectErr: false,
		},
		{
			input: &ListenerRawInput{
				Protocol: "HTTPS",
			},
			expectErr: true,
		},
		{
			input: &ListenerRawInput{
				Protocol: "grpc",
			},
			expectErr: true,
		},
		{
			input: &ListenerRawInput{
				Protocol: "HTTP",
			},
			expectErr: true,
		},
	}

	for i, c := range cases {
		err := c.input.validateProtocol()
		if c.expectErr {
			assert.Error(t, err, "case %d failed", i)
		} else {
			assert.NoError(t, err, "case %d failed", i)
		}
	}

}

func TestListenerRawInput_validateRSType(t *testing.T) {
	cases := []struct {
		input     *ListenerRawInput
		expectErr bool
	}{
		{
			input: &ListenerRawInput{
				InstType: "ENI",
			},
			expectErr: false,
		},
		{
			input: &ListenerRawInput{
				InstType: "CNN",
			},
			expectErr: true,
		},
	}

	for i, c := range cases {
		err := c.input.validateInstType()
		if c.expectErr {
			assert.Error(t, err, "case %d failed", i)
		} else {
			assert.NoError(t, err, "case %d failed", i)
		}
	}
}

func TestListenerRawInput_validateWeight(t *testing.T) {
	cases := []struct {
		input     *ListenerRawInput
		expectErr bool
	}{
		{
			input: &ListenerRawInput{
				Weight: []int{1},
			},
			expectErr: false,
		},
		{
			input: &ListenerRawInput{
				Weight: []int{101},
			},
			expectErr: true,
		},
	}

	for i, c := range cases {
		err := c.input.validateWieght()
		if c.expectErr {
			assert.Error(t, err, "case %d failed", i)
		} else {
			assert.NoError(t, err, "case %d failed", i)
		}
	}
}
