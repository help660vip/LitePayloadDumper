package com.help660.litepayloaddumper;

import android.content.Context;
import android.graphics.Color;
import android.view.Gravity;
import android.view.View;
import android.view.ViewGroup;
import android.widget.BaseAdapter;
import android.widget.CheckBox;
import android.widget.LinearLayout;
import android.widget.TextView;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;

final class PartitionAdapter extends BaseAdapter {
    private final Context context;
    private final Map<String, Boolean> checks;
    private final List<Partition> visible = new ArrayList<>();
    private boolean enabled = true;

    PartitionAdapter(Context context, Map<String, Boolean> checks) {
        this.context = context;
        this.checks = checks;
    }

    void setItems(List<Partition> items, String query) {
        visible.clear();
        for (Partition item : items) {
            if (FormatUtils.matches(item.name, query)) {
                visible.add(item);
            }
        }
        notifyDataSetChanged();
    }

    void setEnabled(boolean value) {
        enabled = value;
        notifyDataSetChanged();
    }

    @Override public int getCount() { return visible.size(); }
    @Override public Partition getItem(int position) { return visible.get(position); }
    @Override public long getItemId(int position) { return position; }

    @Override
    public View getView(int position, View recycled, ViewGroup parent) {
        Holder holder;
        if (recycled == null) {
            LinearLayout row = new LinearLayout(context);
            row.setOrientation(LinearLayout.HORIZONTAL);
            row.setGravity(Gravity.CENTER_VERTICAL);
            row.setPadding(dp(8), dp(7), dp(8), dp(7));

            CheckBox box = new CheckBox(context);
            row.addView(box, new LinearLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT));

            LinearLayout text = new LinearLayout(context);
            text.setOrientation(LinearLayout.VERTICAL);
            TextView title = new TextView(context);
            title.setTextSize(15);
            title.setTextColor(Color.rgb(25, 25, 25));
            TextView detail = new TextView(context);
            detail.setTextSize(12);
            detail.setTextColor(Color.rgb(95, 95, 95));
            text.addView(title);
            text.addView(detail);
            row.addView(text, new LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1));
            holder = new Holder(row, box, title, detail);
            row.setTag(holder);
            recycled = row;
        } else {
            holder = (Holder) recycled.getTag();
        }

        Partition item = getItem(position);
        holder.title.setText(item.name + "  ·  " + FormatUtils.bytes(item.size));
        holder.detail.setText(item.status());
        holder.box.setOnCheckedChangeListener(null);
        holder.box.setChecked(Boolean.TRUE.equals(checks.get(item.name)));
        boolean canCheck = enabled && item.extractable();
        holder.box.setEnabled(canCheck);
        holder.row.setEnabled(canCheck);
        holder.row.setAlpha(canCheck ? 1f : 0.55f);
        holder.box.setOnCheckedChangeListener((button, checked) -> checks.put(item.name, checked));
        holder.row.setOnClickListener(view -> {
            if (canCheck) {
                holder.box.setChecked(!holder.box.isChecked());
            }
        });
        return recycled;
    }

    private int dp(int value) {
        return Math.round(value * context.getResources().getDisplayMetrics().density);
    }

    private static final class Holder {
        final LinearLayout row;
        final CheckBox box;
        final TextView title;
        final TextView detail;

        Holder(LinearLayout row, CheckBox box, TextView title, TextView detail) {
            this.row = row;
            this.box = box;
            this.title = title;
            this.detail = detail;
        }
    }
}
