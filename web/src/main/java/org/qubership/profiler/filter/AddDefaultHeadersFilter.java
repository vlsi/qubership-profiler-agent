package org.qubership.profiler.filter;

import javax.servlet.*;
import javax.servlet.http.HttpServletRequest;
import javax.servlet.http.HttpServletResponse;
import java.io.IOException;

public class AddDefaultHeadersFilter implements Filter {
    public void destroy() {
    }

    public void doFilter(ServletRequest req, ServletResponse resp, FilterChain chain) throws ServletException, IOException {
        if (resp instanceof HttpServletResponse) {
            HttpServletResponse res = (HttpServletResponse) resp;
            res.setHeader("X-Frame-Options", "SAMEORIGIN");
            res.setHeader("X-XSS-Protection", "0");
            res.setHeader("X-content-Type-options", "nosniff");
        }
        chain.doFilter(req, resp);
    }

    public void init(FilterConfig config) throws ServletException {
    }
}
