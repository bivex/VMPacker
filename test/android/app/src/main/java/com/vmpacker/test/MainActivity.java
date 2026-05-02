package com.vmpacker.test;

import android.app.Activity;
import android.os.Bundle;
import android.widget.Button;
import android.widget.ScrollView;
import android.widget.TextView;

public class MainActivity extends Activity {

    private TextView tvResult;

    @Override
    protected void onCreate(Bundle savedInstanceState) {
        super.onCreate(savedInstanceState);
        setContentView(R.layout.activity_main);

        tvResult = findViewById(R.id.tv_result);
        Button btnRun = findViewById(R.id.btn_run);

        btnRun.setOnClickListener(v -> runTests());

        runTests();
    }

    private void runTests() {
        tvResult.setText("Running tests...\n");
        try {
            String result = NativeTest.runAllTests();
            tvResult.setText(result);
        } catch (Throwable t) {
            tvResult.setText("CRASH: " + t.getMessage() + "\n" + stackTraceToString(t));
        }
    }

    private static String stackTraceToString(Throwable t) {
        StringBuilder sb = new StringBuilder();
        for (StackTraceElement e : t.getStackTrace()) {
            sb.append("  at ").append(e.toString()).append('\n');
        }
        return sb.toString();
    }
}
