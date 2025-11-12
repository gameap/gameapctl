package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var scQueryex = `SERVICE_NAME: MessagingService_febe8
DISPLAY_NAME: MessagingService_febe8
        TYPE               : e0  USER_SHARE_PROCESS INSTANCE
        STATE              : 1  STOPPED
        WIN32_EXIT_CODE    : 1077  (0x435)
        SERVICE_EXIT_CODE  : 0  (0x0)
        CHECKPOINT         : 0x0
        WAIT_HINT          : 0x0
        PID                : 0
        FLAGS              :

SERVICE_NAME: NPSMSvc_febe8
DISPLAY_NAME: NPSMSvc_febe8
        TYPE               : f0   ERROR
        STATE              : 4  RUNNING
                                (STOPPABLE, NOT_PAUSABLE, IGNORES_SHUTDOWN)
        WIN32_EXIT_CODE    : 0  (0x0)
        SERVICE_EXIT_CODE  : 0  (0x0)
        CHECKPOINT         : 0x0
        WAIT_HINT          : 0x0
        PID                : 1456
        FLAGS              :

`

func Test_parseScQueryex(t *testing.T) {
	parsed, err := parseScQueryex([]byte(scQueryex))

	require.NoError(t, err)
	require.Len(t, parsed, 2)
	assert.Equal(t, "MessagingService_febe8", parsed[0].ServiceName)
	assert.Equal(t, windowsServiceStateStopped, parsed[0].State)
	assert.Equal(t, "NPSMSvc_febe8", parsed[1].ServiceName)
	assert.Equal(t, windowsServiceStateRunning, parsed[1].State)
}
