package org.qubership.profiler.test.dump;

import org.testng.annotations.Test;

import java.io.BufferedOutputStream;
import java.io.ByteArrayOutputStream;
import java.io.DataOutputStream;
import java.io.IOException;
import java.nio.ByteBuffer;
import java.util.zip.Deflater;
import java.util.zip.DeflaterOutputStream;

public class BufferSpeedTest {

    @Test(invocationCount = 5, groups = "performance")
    public void dataOutputStream() throws IOException {
        ByteArrayOutputStream baos = new ByteArrayOutputStream(100000 * 8);
        DeflaterOutputStream gz = new DeflaterOutputStream(baos);
        DataOutputStream dos = new DataOutputStream(new BufferedOutputStream(gz));

        for (int i = 0; i < 200000; i++) {
            dos.writeLong(i);
        }
        dos.close();
    }

    @Test(invocationCount = 5, groups = "performance")
    public void byteBufferStream() throws IOException {
        ByteArrayOutputStream baos = new ByteArrayOutputStream(100000 * 8);
        final ByteBuffer bb = ByteBuffer.allocate(102400);
        byte[] buf = bb.array();
        DeflaterOutputStream gz = new DeflaterOutputStream(baos, new Deflater(), 102400);
//        DataOutputStream dos = new DataOutputStream(gz);

        for (int i = 0; i < 200000; i++) {
            bb.putLong(i);
            if (bb.remaining() < 10) {
                gz.write(buf, 0, bb.position());
                bb.clear();
            }
        }
        gz.close();
    }

}
