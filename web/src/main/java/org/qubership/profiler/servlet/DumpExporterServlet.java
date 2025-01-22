package org.qubership.profiler.servlet;

import org.qubership.profiler.io.IDumpExporter;
import org.qubership.profiler.io.TemporalRequestParams;
import org.qubership.profiler.io.TemporalUtils;

import javax.servlet.ServletException;
import javax.servlet.http.HttpServlet;
import javax.servlet.http.HttpServletRequest;
import javax.servlet.http.HttpServletResponse;
import java.io.*;

public class DumpExporterServlet extends HttpServlet {


    @Override
    protected void doGet(HttpServletRequest req, HttpServletResponse resp) throws ServletException, IOException {
        IDumpExporter exporter = SpringBootInitializer.dumpExporter();

        exporter.exportDump(req, resp);
    }


}
