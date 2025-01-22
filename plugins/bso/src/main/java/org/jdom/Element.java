package org.jdom;

import java.util.List;

/**
 * org.jdom stub
 */
public class Element {
    private String value;

    public native Element getChild(String processID, Namespace HEADER_PARAMS_NAMESPACE);

    public native Element addContent(String s);

    public native List getContent();

    public native String getText();
}
