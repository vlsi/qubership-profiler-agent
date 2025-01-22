package org.qubership.profiler.sax.raw;

public interface ISuspendLogVisitor {

     void visitHiccup(long date, int delay);

     void visitEnd();

     ISuspendLogVisitor asSkipVisitEnd();
}
