package updater

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_resolveRepeat(t *testing.T) {
	t.Parallel()
	testCases := map[string]struct {
		lookupIPResult [][]net.IP
		lookupIPErr    error
		n              int
		ips            []net.IP
		err            error
	}{
		"failure": {
			lookupIPResult: [][]net.IP{
				{{1, 1, 1, 1}, {1, 1, 1, 2}},
			},
			lookupIPErr: fmt.Errorf("feeling sick"),
			n:           1,
			err:         fmt.Errorf("feeling sick"),
		},
		"successful": {
			lookupIPResult: [][]net.IP{
				{{1, 1, 1, 1}, {1, 1, 1, 2}},
				{{2, 1, 1, 1}, {2, 1, 1, 2}},
				{{2, 1, 1, 3}, {2, 1, 1, 2}},
			},
			n: 3,
			ips: []net.IP{
				{1, 1, 1, 1},
				{1, 1, 1, 2},
				{2, 1, 1, 1},
				{2, 1, 1, 2},
				{2, 1, 1, 3},
			},
		},
	}
	for name, testCase := range testCases {
		testCase := testCase
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if testCase.lookupIPErr == nil {
				require.Len(t, testCase.lookupIPResult, testCase.n)
			}
			const host = "blabla"
			i := 0
			mutex := &sync.Mutex{}
			lookupIP := func(ctx context.Context, argHost string) (
				ips []net.IP, err error) {
				assert.Equal(t, host, argHost)
				mutex.Lock()
				result := testCase.lookupIPResult[i]
				i++
				mutex.Unlock()
				return result, testCase.err
			}

			ips, err := resolveRepeat(
				context.Background(), lookupIP, host, testCase.n, 0)
			if testCase.err != nil {
				require.Error(t, err)
				assert.Equal(t, testCase.err.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, testCase.ips, ips)
		})
	}
}
