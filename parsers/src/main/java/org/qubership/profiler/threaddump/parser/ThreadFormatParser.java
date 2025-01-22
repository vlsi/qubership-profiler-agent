package org.qubership.profiler.threaddump.parser;

/**
 * Interface for thread dump parser
 */
public interface ThreadFormatParser {
    ThreadInfo parseThread(String s);

    ThreaddumpParser.ThreadLineInfo parseThreadLine(String s);
}
