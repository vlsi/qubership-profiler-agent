package org.qubership.profiler.analyzer;

import org.qubership.profiler.dom.ProfiledTree;
import org.qubership.profiler.dom.ProfiledTreeStreamVisitor;
import org.qubership.profiler.util.ProfilerConstants;

import java.util.ArrayList;
import java.util.Collection;

public class DelayTreeTillReady extends ProfiledTreeStreamVisitor {
    private Collection<ProfiledTree> trees = new ArrayList<ProfiledTree>();
    private boolean dictionaryReady;

    public DelayTreeTillReady(ProfiledTreeStreamVisitor sv) {
        this(ProfilerConstants.PROFILER_V1, sv);
    }

    protected DelayTreeTillReady(int api, ProfiledTreeStreamVisitor sv) {
        super(api, sv);
    }

    @Override
    public void visitTree(ProfiledTree tree) {
        if (dictionaryReady)
            super.visitTree(tree);
        else
            trees.add(tree);
    }

    @Override
    public void visitDictionaryReady() {
        dictionaryReady = true;
        for (ProfiledTree tree : trees) {
            super.visitTree(tree);
        }
        trees.clear();
    }
}
