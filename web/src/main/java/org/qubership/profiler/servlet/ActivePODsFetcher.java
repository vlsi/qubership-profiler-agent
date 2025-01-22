package org.qubership.profiler.servlet;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.qubership.profiler.io.ActivePODReport;
import org.qubership.profiler.io.IActivePODReporter;
import org.qubership.profiler.io.TemporalRequestParams;
import org.qubership.profiler.io.TemporalUtils;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import javax.servlet.ServletException;
import javax.servlet.http.HttpServlet;
import javax.servlet.http.HttpServletRequest;
import javax.servlet.http.HttpServletResponse;
import java.io.IOException;
import java.util.List;

public class ActivePODsFetcher extends HttpServlet {
    private static final Logger logger = LoggerFactory.getLogger(ActivePODsFetcher.class);

    @Override
    protected void doPost(HttpServletRequest req, HttpServletResponse resp) throws ServletException, IOException {
        TemporalRequestParams temporal = TemporalUtils.parseTemporal(req);
        String searchConditions = req.getParameter("searchConditions");
        IActivePODReporter reporter = SpringBootInitializer.activePODReporter();
        List<ActivePODReport> report = reporter.reportActivePODs(searchConditions, temporal);
        ObjectMapper objectMapper = new ObjectMapper();
        objectMapper.writeValue(resp.getOutputStream(), report);
    }
}
