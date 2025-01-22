package org.qubership.profiler.chart;

public interface UnaryFunction<A, T> {
    public T evaluate(A arg);
}
