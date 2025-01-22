package org.qubership.profiler;

import org.qubership.profiler.util.cache.TLimitedLongIntHashMap;

public class CallsState {

    public int threadIdsCounter;
    public TLimitedLongIntHashMap threadIdsCache = new TLimitedLongIntHashMap(Integer.getInteger(Dumper.class.getName() + ".THREAD_IDS_CACHE_SIZE", 500));
    int callsTimer;

}
