package org.qubership.profiler.cloud.transport;

public class ProfilerProtocolBlacklistedException extends org.qubership.profiler.cloud.transport.ProfilerProtocolException {

    public ProfilerProtocolBlacklistedException(String message) {
        super(message);
    }

    public ProfilerProtocolBlacklistedException(String message, Throwable cause) {
        super(message, cause);
    }

    public ProfilerProtocolBlacklistedException(Throwable cause) {
        super(cause);
    }
}
