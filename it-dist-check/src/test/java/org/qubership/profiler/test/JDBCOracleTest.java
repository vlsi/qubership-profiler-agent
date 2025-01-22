package org.qubership.profiler.test;

import org.qubership.profiler.agent.LocalState;
import org.qubership.profiler.agent.Profiler;
import org.testng.Assert;
import org.testng.annotations.Test;

import java.math.BigDecimal;
import java.math.BigInteger;
import java.sql.*;
import java.util.Calendar;

public class JDBCOracleTest extends JDBCBaseTest {
    protected String getJDBCUrl() {
        return "db123.qubership.org:1524:RDBL12";
    }

    protected String getJDBCUsername() {
        return "NC_BASE";
    }

    protected String getJDBCPassword() {
        return "NC_BASE";
    }

    protected String getDatabaseName() {
        return "Oracle Database";
    }



    @Test
    public void oracleClientId() throws SQLException {
        Connection con = getConnection("oracleClientId");
        Profiler.enter("void org.qubership.profiler.test.JDBCOracleTest.oracleClientId() (JDBCOracleTest.java:25) [unknown jar]");
        try {
            LocalState state = Profiler.getState();
            state.callInfo.setNcUser("user");
            state.callInfo.setAction("testAction");
            state.callInfo.setEndToEndId("e2e");
            state.callInfo.setCliendId("clientId");
            state.callInfo.setClientInfo("clientInfo");
            PreparedStatement ps = con.prepareStatement("begin dbms_lock.sleep(?); end;");
            try {
                ps.setInt(1, 1);
                ps.execute();
                ps.close();

                ps = con.prepareStatement("select sys_context('USERENV', 'CLIENT_IDENTIFIER') from dual");
                ResultSet rs = ps.executeQuery();
                try {
                    Assert.assertTrue(rs.next(), "dual should have a row");
                    Assert.assertEquals(rs.getString(1), "clientId", "sys_context('USERENV', 'CLIENT_IDENTIFIER') should have value from callInfo.setCliendId");
                } finally {
                    rs.close();
                }
            } finally {
                ps.close();
            }
        } finally {
            Profiler.exit();
            con.close();
        }
    }

    @Test
    public void oracleImprovedSQLException() throws SQLException {
        Connection con = getConnection("oracleImprovedSQLException");
        Profiler.enter("void org.qubership.profiler.test.JDBCOracleTest.oracleImprovedSQLException() (JDBCOracleTest.java:59) [unknown jar]");
        try {
            PreparedStatement ps = con.prepareStatement("declare x number := ?; begin raise_application_error(-20202, 'invalid_statement'); end;");
            try {
                ps.setBigDecimal(1, new BigDecimal("9135259103313548424"));
                ps.execute();
            } finally {
                ps.close();
            }
            Assert.fail("Invalid statement should cause SQLException");
        } catch (SQLException e) {
            String message = e.getMessage();
            if (message == null || !message.contains("SQL: declare")) {
                Assert.fail("SQLException should include sql text. Actual message: " + message);
            }
            if (!message.contains("9135259103313548424")) {
                Assert.fail("SQLException should include bind variables (9135259103313548424). Actual message: " + message);
            }
        } finally {
            Profiler.exit();
            con.close();
        }
    }

    @Test
    public void oracleBadSql() throws SQLException {
        Connection con = getConnection("oracleBadSql");
        Profiler.enter("void org.qubership.profiler.test.JDBCOracleTest.oracleBadSql() (JDBCOracleTest.java:87) [unknown jar]");
        String sql = "select from ${table} where ${where}";
        try {
            PreparedStatement ps = con.prepareStatement(sql);
            try {
                ps.execute();
            } finally {
                ps.close();
            }
            Assert.fail("Invalid statement should cause SQLException");
        } catch (SQLException e) {
            String msg = e.getMessage();
            if (msg == null) {
                throw e;
            }
            Assert.assertTrue(msg.contains(sql), "Exception message should contain <<" + sql + ">>, actual message was <<" + msg + ">>");
        } finally {
            Profiler.exit();
            con.close();
        }
    }

    // Binders tests

    // Nulls

    @Test
    public void testBindNull() throws Exception {
        checkBindByException("testBindNull", Binders.nullBinder(Types.VARCHAR), "null");
        checkBindByException("testBindNull", Binders.chain(Binders.stringBinder("some_value"), Binders.nullBinder(Types.VARCHAR)), "null");
    }

    @Test
    public void testBindNullObject() throws Exception {
        checkBindByException("testBindNullObject", Binders.object(null), "null");
        checkBindByException("testBindNullObject", Binders.chain(Binders.stringBinder("some_value"), Binders.object(null)), "null");
    }

    // Primitive type binds

    @Test
    public void testBindBoolean() throws Exception {
        checkBindByException("testBindBoolean", Binders.booleanBinder(true), "1");
        checkBindByException("testBindBoolean", Binders.booleanBinder(false), "0");
    }

    @Test
    public void testBindByte() throws Exception {
        checkBindByException("testBindByte", Binders.byteBinder((byte) 42), "42");
        checkBindByException("testBindByte", Binders.byteBinder((byte) 101), "101");
    }

    @Test
    public void testBindShort() throws Exception {
        checkBindByException("testBindShort", Binders.shortBinder((short) 42), "42");
        checkBindByException("testBindShort", Binders.shortBinder((short) 538), "538");
    }

    @Test
    public void testBindInt() throws Exception {
        checkBindByException("testBindInt", Binders.intBinder(42), "42");
        checkBindByException("testBindInt", Binders.intBinder(65537), "65537");
        checkBindByException("testBindInt", Binders.intBinder(Integer.MAX_VALUE), String.valueOf(Integer.MAX_VALUE));
    }

    @Test
    public void testBindLong() throws Exception {
        checkBindByException("testBindLong", Binders.longBinder(42L), "42");
        checkBindByException("testBindLong", Binders.longBinder(Long.MAX_VALUE), String.valueOf(Long.MAX_VALUE));
    }

    @Test
    public void testBindFloat() throws Exception {
        checkBindByException("testBindFloat", Binders.floatBinder(0.5f), "0.5");
    }

    @Test
    public void testBindDouble() throws Exception {
        checkBindByException("testBindDouble", Binders.doubleBinder(0.5d), "0.5");
    }

    // Primitive type wrappers

    @Test
    public void testBindBooleanObject() throws Exception {
        checkBindByException("testBindBooleanObject", Binders.object(Boolean.TRUE), "1");
        checkBindByException("testBindBooleanObject", Binders.object(Boolean.FALSE), "0");
    }

    @Test
    public void testBindByteObject() throws Exception {
        checkBindByException("testBindByteObject", Binders.object((byte) 42), "42");
        checkBindByException("testBindByteObject", Binders.object((byte) 101), "101");
    }

    @Test
    public void testBindShortObject() throws Exception {
        checkBindByException("testBindShortObject", Binders.object((short) 42), "42");
        checkBindByException("testBindShortObject", Binders.object((short) 538), "538");
    }

    @Test
    public void testBindIntObject() throws Exception {
        checkBindByException("testBindIntObject", Binders.object(42), "42");
        checkBindByException("testBindIntObject", Binders.object(65537), "65537");
        checkBindByException("testBindIntObject", Binders.object(Integer.MAX_VALUE), String.valueOf(Integer.MAX_VALUE));
    }

    @Test
    public void testBindLongObject() throws Exception {
        checkBindByException("testBindLongObject", Binders.object(42), "42");
        checkBindByException("testBindLongObject", Binders.object(Long.MAX_VALUE), String.valueOf(Long.MAX_VALUE));
    }

    @Test
    public void testBindFloatObject() throws Exception {
        checkBindByException("testBindFloatObject", Binders.object(0.5f), "0.5");
    }

    @Test
    public void testBindDoubleObject() throws Exception {
        checkBindByException("testBindDoubleObject", Binders.object(0.5d), "0.5");
    }

    // Simple types

    @Test
    public void testBindString() throws Exception {
        checkBindByException("testBindString", Binders.stringBinder("test_value"), "'test_value'");
    }

    @Test
    public void testBindStringObject() throws Exception {
        checkBindByException("testBindStringObject", Binders.object("test_value"), "'test_value'");
    }

    @Test
    public void testBindBigDecimal() throws Exception {
        checkBindByException("testBindBigDecimal", Binders.bigDecimalBinder(new BigDecimal(new BigInteger("9135259103313548424"))), "9135259103313548424");
    }

    @Test
    public void testBindBidDecimalObject() throws Exception {
        checkBindByException("testBindBigDecimalObject", Binders.object(new BigDecimal(new BigInteger("9135259103313548424"))), "9135259103313548424");
    }

    @Test
    public void testBindTime() throws Exception {
        Time value = new Time(System.currentTimeMillis());
        checkBindByException("testBindTime", Binders.timeBinder(value), value.toString());
    }

    @Test
    public void testBindTimeObject() throws Exception {
        Time value = new Time(System.currentTimeMillis());
        checkBindByException("testBindTimeObject", Binders.object(value), value.toString());
    }

    @Test
    public void testBindTimeWithCalendar() throws Exception {
        Time value = new Time(System.currentTimeMillis());
        Calendar calendar = Calendar.getInstance();
        checkBindByException("testBindTimeWithCalendar", Binders.timeBinder(value, calendar), value.toString());
    }

    @Test
    public void testBindTimestamp() throws Exception {
        Timestamp value = new Timestamp(System.currentTimeMillis());
        checkBindByException("testBindTimestamp", Binders.timestampBinder(value), value.toString());
    }

    @Test
    public void testBindTimestampObject() throws Exception {
        Timestamp value = new Timestamp(System.currentTimeMillis());
        checkBindByException("testBindTimestampObject", Binders.object(value), value.toString());
    }

    @Test
    public void testBindTimestampWithCalendar() throws Exception {
        Timestamp value = new Timestamp(System.currentTimeMillis());
        Calendar calendar = Calendar.getInstance();
        checkBindByException("testBindTimestampWithCalendar", Binders.timestampBinder(value, calendar), value.toString());
    }

    @Test
    public void testBindDate() throws Exception {
        Date value = new Date(System.currentTimeMillis());
        checkBindByException("testBindDate", Binders.dateBinder(value), value.toString());
    }

    @Test
    public void testBindDateObject() throws Exception {
        Date value = new Date(System.currentTimeMillis());
        checkBindByException("testBindDateObject", Binders.object(value), value.toString());
    }

    @Test
    public void testBindDateWithCalendar() throws Exception {
        Date value = new Date(System.currentTimeMillis());
        Calendar calendar = Calendar.getInstance();
        checkBindByException("testBindDateWithCalendar", Binders.dateBinder(value, calendar), value.toString());
    }

    private Array createArray(Connection con, String typeName, Object values) {
        try {
            return (Array) Class.forName("oracle.jdbc.OracleConnection").getMethod("createOracleArray", String.class, Object.class).invoke(con, typeName, values);
        } catch (Exception e) {
            throw new RuntimeException(e);
        }
    }

    private Struct createStruct(Connection con, String typeName, Object[] values) {
        try {
            return (Struct) Class.forName("oracle.jdbc.OracleConnection").getMethod("createStruct", String.class, Object[].class).invoke(con, typeName, values);
        } catch (Exception e) {
            throw new RuntimeException(e);
        }
    }

    @Test
    public void testBindArray() throws Exception {
        final BigDecimal[] value = new BigDecimal[]{new BigDecimal("1"), new BigDecimal("2"), new BigDecimal("3")};
        checkBindByException("testBindArray", new Binder() {
            public void bind(PreparedStatement ps, int column) throws SQLException {
                ps.setArray(column, createArray(ps.getConnection(), "ARRAYOFNUMBERS", value));
            }
        }, "ARRAYOFNUMBERS(1, 2, 3)");
    }

    @Test
    public void testBindArrayObject() throws Exception {
        final BigDecimal[] value = new BigDecimal[]{new BigDecimal("1"), new BigDecimal("2"), new BigDecimal("3")};
        checkBindByException("testBindArrayObject", new Binder() {
            public void bind(PreparedStatement ps, int column) throws SQLException {
                ps.setObject(column, createArray(ps.getConnection(), "ARRAYOFNUMBERS", value));
            }
        }, "ARRAYOFNUMBERS(1, 2, 3)");
    }

    @Test
    public void testBindStruct() throws Exception {
        final Object[] value = new Object[]{new BigDecimal("42"), "test_value"};
        checkBindByException("testBindStruct", new Binder() {
            public void bind(PreparedStatement ps, int column) throws SQLException {
                ps.setObject(column, createStruct(ps.getConnection(), "NUMANDSTR", value));
            }
        }, "NUMANDSTR(42, 'test_value')");
    }

    @Test
    public void testBindCompoundObject() throws Exception {
        checkBindByException("testBindCompoundObject", new Binder() {
            public void bind(PreparedStatement ps, int column) throws SQLException {
                Connection con = ps.getConnection();
                Object value = createArray(con, "TABLEOFNUMANDSTR", new Object[]{
                        createStruct(con, "NUMANDSTR", new Object[]{new BigDecimal("1"), "first"}),
                        createStruct(con, "NUMANDSTR", new Object[]{new BigDecimal("2"), "second"})
                });
                ps.setObject(column, value);
            }
        }, "TABLEOFNUMANDSTR(NUMANDSTR(1, 'first'), NUMANDSTR(2, 'second'))");
    }

    // Service functions

    @Test
    public void testBindClear() throws Exception {
        checkBindByException("testBindClear", Binders.chain(Binders.stringBinder("some_value"), Binders.clear()), "some_value", false);
    }

    @Test
    public void testBindAddBatch() throws Exception {
        String testName = "testBindAddBatch";
        Connection con = getConnection(testName);
        try {
            PreparedStatement ps = con.prepareStatement("insert into nc$_ids values (?)");
            try {
                ps.setInt(1, 1);
                ps.addBatch();
                ps.setInt(1, 2);
                ps.addBatch();
                ps.setInt(1, 3);
                int[] res = ps.executeBatch();
                Assert.assertNotNull(res, "Batch result shouldn't be null");
                Assert.assertEquals(res.length, 2, "Batch should contain two statements");
                for (int i : res) {
                    Assert.assertTrue( i== 1 || i == Statement.SUCCESS_NO_INFO, "Each update in batch should insert one row");
                }
                int singleUpdate = ps.executeUpdate();
                Assert.assertEquals(singleUpdate, 1, "Single update should add one row");
            } catch (SQLException e) {
                Assert.fail("Unexpected exception", e);
            } finally {
                ps.close();
            }
        } finally {
            con.close();
        }
    }

    @Test
    public void testBindClearBatch() throws Exception {
        String testName = "testBindClearBatch";
        Connection con = getConnection(testName);
        try {
            PreparedStatement ps = con.prepareStatement("insert into some_table$ values (?)");
            try {
                try {
                    ps.setString(1, "some_value");
                    ps.addBatch();
                    // addBatch doesn't clear current parameters, so next addBatch is valid
                    ps.addBatch();
                    ps.clearBatch(); // This call will clear all batched parameters AND current parameters
                } catch (SQLException e) {
                    Assert.fail("Unexpected exception", e);
                }
                ps.addBatch(); // Should fail here
                Assert.fail("PreparedStatement.addBatch must fail if not all variables are bound");
            } catch (SQLException e) {
                checkExceptionMessageForBinds(testName, e, false, false, "some_value");
            } finally {
                ps.close();
            }
        } finally {
            con.close();
        }
    }
}
