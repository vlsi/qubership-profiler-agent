package com.netcracker.profiler.sax.readers;

import com.netcracker.profiler.dump.DataInputStreamEx;
import com.netcracker.profiler.guice.DumpRootLocation;
import com.netcracker.profiler.io.ParamReader;
import com.netcracker.profiler.io.ParamReaderFileFactory;
import com.netcracker.profiler.sax.raw.ClobReaderFlyweight;
import com.netcracker.profiler.sax.raw.ClobReaderFlyweightFile;
import com.netcracker.profiler.sax.raw.SuspendLogVisitor;

import java.io.File;
import java.io.IOException;

import jakarta.inject.Inject;

public class ProfilerTraceReaderFile extends ProfilerTraceReader {

    private final File dumpRoot;

    @Inject
    public ProfilerTraceReaderFile(
            @DumpRootLocation File dumpRoot,
            ParamReaderFileFactory paramReaderFileFactory) {
        super(null, null, paramReaderFileFactory);
        this.dumpRoot = dumpRoot;
    }

    private File dataFolder(){
        return new File(dumpRoot, rootReference);
    }

    protected SuspendLogReader suspendLogReader(SuspendLogVisitor sv, long begin, long end) {
        // Create directly instead of using ApplicationContext
        return new SuspendLogReader(sv, dataFolder().getAbsolutePath(), begin, end);
    }

    protected SuspendLogReader suspendLogReader(SuspendLogVisitor sv) {
        return suspendLogReader(sv, Long.MIN_VALUE, Long.MAX_VALUE);
    }

    public DataInputStreamEx reopenDataInputStream(DataInputStreamEx oldOne, String streamName, int traceFileIndex) throws IOException {
        return DataInputStreamEx.reopenDataInputStream(oldOne, dataFolder(), streamName, traceFileIndex);
    }

    @Override
    public ClobReaderFlyweight clobReaderFlyweight() {
        ClobReaderFlyweightFile result = new ClobReaderFlyweightFile();
        result.setDataFolder(dataFolder());
        return result;
    }

    protected ParamReader paramReader(){
        return paramReaderFileFactory.create(dataFolder());
    }
}
