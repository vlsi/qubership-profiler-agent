package org.qubership.profiler.tools;

import java.io.*;
import java.util.ArrayList;
import java.util.List;
import java.util.Date;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

public class MergeCSSandJS {
    public static final String CSS = "<link type=\"text/css\" href=\"([^\"]+?\\.css)\" rel=\"stylesheet\"/>\\s*";
    public static final String JS = "<script type=\"text/javascript\" language=\"JavaScript\" src=\"([^\"]+?\\.js)\"></script>\\s*";

    public static final Pattern PATTERN_CSS = Pattern.compile(CSS);
    public static final Pattern PATTERN_JS = Pattern.compile(JS);

    private static long getLastModifiedTime(File file) {
        long fileLastModified = file.lastModified();

        if (fileLastModified == 0L) {
            fileLastModified = System.currentTimeMillis();
        }
        return fileLastModified;
    }

    private static void collectItems(File root, String dst, String source, Pattern regex) throws IOException {
        long lastRun = getLastModifiedTime(new File(dst));
        long lastModified = 0;
        Matcher m = regex.matcher(source);
        while (m.find()) {
            final String child = m.group(1);
            File item = new File(root.getParentFile(), child);
            lastModified = Math.max(lastModified, getLastModifiedTime(item));
        }

        if (new File(dst).exists() && lastRun > lastModified) {
            System.out.println("Skipped rewriting " + root + " -> " + dst + " since file is not modified");
            return;
        }

        final FileOutputStream os = new FileOutputStream(dst);
        m = regex.matcher(source);
        while (m.find()) {
            final String child = m.group(1);
            File item = new File(root.getParentFile(), child);
            final byte[] bytes = readFile(item);
            os.write(bytes);
            os.write(13);
        }
        os.close();
    }

    public static void main(String[] args) throws IOException {
        String srcFile = args.length <= 0 || args[0] == null ? "index.html" : args[0];
        String cssFile = args.length <= 1 || args[1] == null ? "css.css" : args[1];
        String jsFile = args.length <= 2 || args[2] == null ? "js.js" : args[2];
        String htmlFile = args.length <= 3 || args[3] == null ? "html.html" : args[3];

        File root = new File(srcFile);
        byte[] buf = readFile(root);

        String contents = new String(buf);

        collectItems(root, cssFile, contents, PATTERN_CSS);
        collectItems(root, jsFile, contents, PATTERN_JS);

        contents = contents.replaceFirst(CSS, "@COMPILED_CSS@");
        contents = contents.replaceFirst(JS, "@COMPILED_JS@");
        contents = contents.replaceAll(CSS, "");
        contents = contents.replaceAll(JS, "");

        contents = contents.replaceFirst("@COMPILED_CSS@", "<link type=\"text/css\" href=\"@CSS@\" rel=\"stylesheet\"/>\n");
        contents = contents.replaceFirst("@COMPILED_JS@", "<script type=\"text/javascript\" language=\"JavaScript\" src=\"@JS@\"></script>\n");

        long lastRun = getLastModifiedTime(new File(htmlFile));
        if (new File(htmlFile).exists() && lastRun > getLastModifiedTime(root)) {
            System.out.println("Skipped rewriting " + srcFile + " -> " + htmlFile + " since file is not modified");
            return;
        }

        FileOutputStream os = new FileOutputStream(htmlFile);
        os.write(contents.getBytes());
        os.close();
    }

    private static byte[] readFile(String arg) throws IOException {
        return readFile(new File(arg));
    }

    private static byte[] readFile(File file) throws IOException {
        InputStream is = null;
        try {
            is = new FileInputStream(file);
            return readFully(is);
        } finally {
            if (is != null) is.close();
        }
    }

    private static byte[] readFully(InputStream is) throws IOException {
        ArrayList<byte[]> os = null;
        byte[] b, out = null;
        int totalLength = 0;
        while (true) {
            b = new byte[Math.max(is.available(), 1000)];
            final int bytesRead = is.read(b);
            if (bytesRead == -1) break;
            totalLength += bytesRead;
            if (bytesRead < b.length) {
                byte[] c = new byte[bytesRead];
                System.arraycopy(b, 0, c, 0, bytesRead);
                b = c;
            }
            if (out == null) {
                out = b;
                continue;
            }

            if (os == null) {
                os = new ArrayList<byte[]>();
                os.add(out);
            }
            os.add(b);
        }
        if (os == null)
            return out;

        b = new byte[totalLength];
        int pos = 0;
        for (byte[] o : os) {
            System.arraycopy(o, 0, b, pos, o.length);
            pos += o.length;
        }

        return b;
    }

    public static List<String> readAllLinesFromFile(File file) throws IOException {
        try(BufferedReader reader = new BufferedReader(new FileReader(file))) {
            List<String> lines = new ArrayList<>();
            String line;
            while((line = reader.readLine())!=null) {
                lines.add(line);
            }
            return lines;
        }
    }
}
