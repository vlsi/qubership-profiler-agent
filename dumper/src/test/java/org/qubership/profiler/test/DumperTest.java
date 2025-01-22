package org.qubership.profiler.test;

import org.testng.Assert;
import org.testng.annotations.Test;

import java.io.*;
import java.nio.ByteBuffer;
import java.nio.CharBuffer;
import java.nio.charset.Charset;
import java.nio.charset.CharsetEncoder;

public class DumperTest {
    @Test(enabled = false)
    public static void testByteConversion() throws IOException {
        ByteArrayOutputStream baos = new ByteArrayOutputStream();
        DataOutputStream dos = new DataOutputStream(baos);
        byte x = (byte) (3 << 6);
        System.out.println("x = " + x);
        dos.write(x);
        final byte[] buf = baos.toByteArray();

        ByteArrayInputStream is = new ByteArrayInputStream(buf);
        DataInputStream dis = new DataInputStream(is);
        Assert.assertEquals(dis.read() >> 6, 3);
    }

    @Test(enabled = false)
    public static void testIntConversion() throws IOException {
        ByteArrayOutputStream baos = new ByteArrayOutputStream();
        DataOutputStream dos = new DataOutputStream(baos);
        for (int millis = -10000; millis <= 10000; millis++) {
            baos.reset();

            int b1 = (millis >>> 24);
            int b2 = ((millis >>> 16) & 0xff);
            int b3 = ((millis >>> 8) & 0xff);
            int b4 = (millis & 0xff);
            int cnt = (byte) (b1 != 0 ? 3 : (b2 != 0 ? 2 : (b3 != 0 ? 1 : 0)));
            System.out.println("millis = " + millis);
            System.out.println("cnt = " + cnt);
            cnt |= 1 << 2;
            System.out.println("cnt = " + cnt);
            final int cnn = ((byte) 1) | (cnt << 4);
            System.out.println("cnn = " + cnn);
            System.out.println("cnn>>4 = " + ((cnn >> 4) & 0x3));
            System.out.println("b1 = " + b1);
            System.out.println("b2 = " + b2);
            System.out.println("b3 = " + b3);
            System.out.println("b4 = " + b4);
            dos.writeByte(cnn);
            switch (cnt & 0x3) {
                case 3:
                    dos.write(b1);
                case 2:
                    dos.write(b2);
                case 1:
                    dos.write(b3);
                case 0:
                    dos.write(b4);
            }

            final byte[] buf = baos.toByteArray();
            ByteArrayInputStream is = new ByteArrayInputStream(buf);
            DataInputStream dis = new DataInputStream(is);

            int x = dis.readByte();
            System.out.println("x = " + x);
            int t = 0;
            switch ((x >>> 4) & 0x3) {
                case 3:
                    t |= dis.read() << 24;
                case 2:
                    t |= dis.read() << 16;
                case 1:
                    t |= dis.read() << 8;
                case 0:
                    t |= dis.read();
            }


            Assert.assertEquals(millis, t);
        }
    }

    static final String src = "/**\n" +
            "     * Flushes this encoder.\n" +
            "     *\n" +
            "     * <p> Some encoders maintain internal state and may need to write some\n" +
            "     * final bytes to the output buffer once the overall input sequence has\n" +
            "     * been read.\n" +
            "     *\n" +
            "     * <p> Any additional output is written to the output buffer beginning at\n" +
            "     * its current position.  At most {@link Buffer#remaining out.remaining()}\n" +
            "     * bytes will be written.  The buffer's position will be advanced\n" +
            "     * appropriately, but its mark and limit will not be modified.\n" +
            "     *\n" +
            "     * <p> If this method completes successfully then it returns {@link\n" +
            "     * CoderResult#UNDERFLOW}.  If there is insufficient room in the output\n" +
            "     * buffer then it returns {@link CoderResult#OVERFLOW}.  If this happens\n" +
            "     * then this method must be invoked again, with an output buffer that has\n" +
            "     * more room, in order to complete the current <a href=\"#steps\">encoding\n" +
            "     * operation</a>.\n" +
            "     *\n" +
            "     * <p> If this encoder has already been flushed then invoking this method\n" +
            "     * has no effect.\n" +
            "     *\n" +
            "     * <p> This method invokes the {@link #implFlush implFlush} method to\n" +
            "     * perform the actual flushing operation.  </p>\n" +
            "     *\n" +
            "     * @param  out\n" +
            "     *         The output byte buffer\n" +
            "     *\n" +
            "     * @return  A coder-result object, either {@link CoderResult#UNDERFLOW} or\n" +
            "     *          {@link CoderResult#OVERFLOW}\n" +
            "     *\n" +
            "     * @throws  IllegalStateException\n" +
            "     *          If the previous step of the current encoding operation was an\n" +
            "     *          invocation neither of the {@link #flush flush} method nor of\n" +
            "     *          the three-argument {@link\n" +
            "     *          #encode(CharBuffer,ByteBuffer,boolean) encode} method\n" +
            "     *          with a value of <tt>true</tt> for the <tt>endOfInput</tt>\n" +
            "     *          parameter\n" +
            "     */";

    @Test(groups = "performance")
    public static void encoder1() {
        Charset cs = Charset.forName("UTF-8");
        CharBuffer cb = CharBuffer.allocate(src.length());
        ByteBuffer bb;
        CharsetEncoder ce = cs.newEncoder();
//        while(true){
        long t = System.nanoTime();
        src.getChars(0, src.length(), cb.array(), 0);
        cb.clear();
        cs.encode(cb);
        t = System.nanoTime() - t - 1300;
        System.out.println("t = " + (t));
//        }
    }
}
