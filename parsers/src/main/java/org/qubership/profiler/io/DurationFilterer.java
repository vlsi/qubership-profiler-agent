package org.qubership.profiler.io;

public interface DurationFilterer extends CallFilterer {
    long getDurationFrom();

    long getDurationTo();
}
