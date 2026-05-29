// (#66) Dashboard + Campaigns 통합 (옵션 C).
// /campaigns 페이지 폐기 → Dashboard 가 단일 진입점.
// - toolbar: [+ New Campaign] [⚙ 캠페인 선택]
// - Actions: View Results / Copy / Delete (owner-only Copy/Delete, viewer 는 shared 캠페인에서 숨김)
// - 차트: 필터(DB 저장) + DataTables 검색 모두 적용된 가시 행만 집계
// - 필터: /api/users/me/preferences 의 dashboard_campaign_filter (JSON 배열) 저장

/* ============================================================
 * 1) Status / Stats metadata
 * ============================================================ */

var campaigns = []        // 전체 캠페인 (summary 응답 원본)
var campaign = {}         // launch() 후 단건 캠페인 (campaigns.js 호환)
var campaignTable = null  // DataTable 인스턴스
var allowedCampaignIds = null  // 필터: null=전체 / Set=선택된 id 만 / 빈 Set=전체로 간주
var filterDraftIds = null      // 모달 내 임시 선택 상태 (적용 전)

var statuses = {
    "Sent": {       color: "#1abc9c", label: "label-success",  icon: "fa-envelope",              point: "ct-point-sent" },
    "In progress":  { label: "label-primary" },
    "Queued":       { label: "label-info" },
    "Completed":    { label: "label-success" },
    "Opened":       { color: "#f9bf3b", label: "label-warning", icon: "fa-envelope",              point: "ct-point-opened" },
    "Reported":     { color: "#45d6ef", label: "label-warning", icon: "fa-bullhorn",              point: "ct-point-reported" },
    "Clicked":      { color: "#F39C12", label: "label-clicked", icon: "fa-mouse-pointer",         point: "ct-point-clicked" },
    "Success":      { color: "#f05b4f", label: "label-danger",  icon: "fa-exclamation",           point: "ct-point-clicked" },
    "Error":        { color: "#6c7a89", label: "label-default", icon: "fa-times",                 point: "ct-point-error" },
    "Error Sending Email": { color: "#6c7a89", label: "label-default", icon: "fa-times",          point: "ct-point-error" },
    "Submitted":    { color: "#f05b4f", label: "label-danger",  icon: "fa-exclamation",           point: "ct-point-clicked" },
    "Unknown":      { color: "#6c7a89", label: "label-default", icon: "fa-question",              point: "ct-point-error" },
    "Sending":      { color: "#428bca", label: "label-primary", icon: "fa-spinner",               point: "ct-point-sending" },
    "Campaign Created": { label: "label-success", icon: "fa-rocket" },
    "Executed":     { color: "#ff0000", label: "label-danger",  icon: "fa-exclamation-triangle",  point: "ct-point-executed" },
    "Trained":      { color: "#2727dd", label: "label-info",    icon: "fa-graduation-cap",        point: "ct-point-trained" }
};

var statsMapping = {
    "sent":      "Sent",
    "opened":    "Opened",
    "reported":  "Reported",
    "clicked":   "Clicked",
    "submitted": "Submitted",
    "executed":  "Executed",
    "trained":   "Trained"
};

/* ============================================================
 * 2) 공통 헬퍼
 * ============================================================ */

function getStat(c, key) {
    var v = c && c.stats ? c.stats[key] : undefined;
    return (typeof v === 'number' && isFinite(v)) ? v : 0;
}

// (#47) 도넛 차트 cell 폭에 따라 폰트 크기 결정.
function pickFontSizes(w) {
    if (w >= 160) return { title: 16, center: 16 };
    if (w >= 120) return { title: 14, center: 14 };
    if (w >= 90)  return { title: 12, center: 12 };
    return { title: 10, center: 10 };
}

// 분(min) 단위 숫자를 "Xh Ym" 또는 "Ym" 포맷으로 변환.
function formatElapsed(totalMin) {
    var mins = Math.floor(totalMin);
    if (mins < 0) mins = 0;
    var h = Math.floor(mins / 60);
    var m = mins % 60;
    if (h <= 0) return m + 'm';
    return h + 'h ' + m + 'm';
}

/* ============================================================
 * 3) 차트
 * ============================================================ */

function renderPieChart(chartopts) {
    if (!document.getElementById(chartopts['elemId'])) return null;
    return Highcharts.chart(chartopts['elemId'], {
        chart: {
            type: 'pie',
            events: {
                load: function () {
                    var chart = this,
                        rend = chart.renderer,
                        pie = chart.series[0],
                        left = chart.plotLeft + pie.center[0],
                        top = chart.plotTop + pie.center[1],
                        sizes = pickFontSizes(chart.chartWidth);
                    chart.title && chart.title.css({ fontSize: sizes.title + 'px' });
                    this.innerText = rend.text(chartopts['data'][0].count, left, top).attr({
                        'text-anchor': 'middle',
                        'dominant-baseline': 'central',
                        'font-size': sizes.center + 'px',
                        'font-weight': 'bold',
                        'fill': chartopts['colors'][0],
                        'font-family': 'Helvetica,Arial,sans-serif'
                    }).add();
                },
                render: function () {
                    if (!this.innerText) return;
                    var pie = this.series[0];
                    if (!pie || !pie.center) return;
                    var sizes = pickFontSizes(this.chartWidth);
                    this.title && this.title.css({ fontSize: sizes.title + 'px' });
                    this.innerText.attr({
                        text: chartopts['data'][0].count,
                        x: this.plotLeft + pie.center[0],
                        y: this.plotTop + pie.center[1],
                        'font-size': sizes.center + 'px'
                    });
                }
            }
        },
        title: { text: chartopts['title'] },
        plotOptions: { pie: { innerSize: '80%', dataLabels: { enabled: false } } },
        credits: { enabled: false },
        tooltip: {
            formatter: function () {
                if (this.key == undefined) return false;
                return '<span style="color:' + this.color + '">\u25CF</span>' + this.point.name + ': <b>' + this.y + '%</b><br/>';
            }
        },
        series: [{ data: chartopts['data'], colors: chartopts['colors'] }]
    });
}

function generateStatsPieCharts(srcCampaigns) {
    var buckets = { sent: 0, opened: 0, clicked: 0, submitted: 0, executed: 0, reported: 0, trained: 0 };
    var total = 0;

    $.each(srcCampaigns, function (i, c) {
        var stats = c.stats || {};
        total += (typeof stats.total === 'number' && isFinite(stats.total)) ? stats.total : 0;
        buckets.sent      += getStat(c, 'sent');
        buckets.opened    += getStat(c, 'opened');
        buckets.clicked   += getStat(c, 'clicked');
        buckets.submitted += getStat(c, 'submitted');
        buckets.executed  += getStat(c, 'executed');
        buckets.reported  += getStat(c, 'reported');
        buckets.trained   += getStat(c, 'trained');
    });

    Object.keys(statsMapping).forEach(function (statusKey) {
        var status_label = statsMapping[statusKey];
        var count = buckets[statusKey] || 0;
        var denom = total || 1;
        var pct = Math.round((count / denom) * 1000) / 10;
        renderPieChart({
            elemId: statusKey + '_chart',
            title: status_label,
            name: statusKey,
            data: [
                { name: status_label, y: pct, count: count },
                { name: '', y: 100 - pct }
            ],
            colors: [(statuses[status_label] && statuses[status_label].color) || '#95a5a6', '#dddddd']
        });
    });
}

// (#49) Click Rate Over Time — 캠페인별 누적 클릭률 라인.
function generateClickRateOverTimeChart(srcCampaigns) {
    if (!document.getElementById('click_rate_over_time_chart')) return;

    // 최근 10개 (created_date desc) 선별 — srcCampaigns 가 이미 가시 캠페인만 들어옴.
    var recent = srcCampaigns.slice().sort(function (a, b) {
        return new Date(b.created_date) - new Date(a.created_date);
    }).slice(0, 10);

    var series = [];
    $.each(recent, function (i, c) {
        var timeline = c.click_timeline || [];
        var total = (c.stats && typeof c.stats.total === 'number') ? c.stats.total : 0;
        if (timeline.length === 0 || total === 0) return true;

        var launchMs = new Date(c.launch_date).getTime();
        var points = [{ x: 0, y: 0, original_time: c.launch_date }];
        $.each(timeline, function (j, pt) {
            var elapsedMin = (new Date(pt.time).getTime() - launchMs) / 60000;
            if (elapsedMin < 0) elapsedMin = 0;
            var rate = (pt.count / total) * 100;
            points.push({
                x: elapsedMin,
                y: Math.round(rate * 100) / 100,
                original_time: pt.time
            });
        });
        series.push({ name: c.name, data: points });
    });

    Highcharts.chart('click_rate_over_time_chart', {
        chart: { type: 'line', zoomType: 'x' },
        title: { text: 'Click Rate Over Time (' + series.length + ' Campaigns)' },
        xAxis: {
            title: { text: 'Elapsed time since launch' },
            min: 0,
            labels: { formatter: function () { return formatElapsed(this.value); } }
        },
        yAxis: { min: 0, ceiling: 100, softMax: 10, title: { text: 'Cumulative Click Rate (%)' } },
        tooltip: {
            shared: false,
            formatter: function () {
                var t = this.point && this.point.original_time
                    ? moment.utc(this.point.original_time).local().format('YYYY-MM-DD HH:mm')
                    : '-';
                return '<b>' + this.series.name + '</b><br>'
                    + 'Time: ' + t + '<br>'
                    + 'Elapsed: ' + formatElapsed(this.x) + '<br>'
                    + 'Click Rate: <b>' + this.y + '%</b>';
            }
        },
        legend: {
            enabled: true,
            useHTML: true,
            itemStyle:      { fontSize: '11px' },
            itemHoverStyle: { color: '#000000' },
            labelFormatter: function () {
                var c = this.visible === false ? '#bbbbbb' : this.color;
                return '<span style="color:' + c + '">' + this.name + '</span>';
            }
        },
        plotOptions: {
            line: {
                marker: { enabled: true, radius: 2, symbol: 'circle' },
                lineWidth: 2,
                states: { hover: { lineWidth: 3 } }
            },
            series: {
                events: {
                    legendItemClick: function () {
                        var chart = this.chart;
                        setTimeout(function () { chart.legend.render(); }, 0);
                    }
                }
            }
        },
        colors: ['#2980b9', '#e67e22', '#27ae60', '#8e44ad', '#c0392b',
                 '#16a085', '#d35400', '#2c3e50', '#7f8c8d', '#f39c12'],
        credits: { enabled: false },
        series: series
    });
}

/* ============================================================
 * 4) Row 렌더링
 * ============================================================ */

function buildActionsCell(c, idx) {
    // (#66) Actions 버튼 한 줄 배치 + 미세 간격(4px) 으로 시각적 분리.
    //       의미에 맞는 색상:
    //         View Results = btn-info     (정보 조회)
    //         Copy         = btn-default  (보조)
    //         Delete       = btn-danger   (위험)
    //       viewer 는 Copy/Delete 를 미표시 대신 disabled 로 노출해
    //       "왜 안 보이지?" 혼동을 줄이고 권한 부재를 명시한다.
    var btnStyle = "margin-left:4px;";
    var html = "<div class='text-left' style='white-space:nowrap;'>"
        + "<a class='btn btn-info' href='/campaigns/" + c.id + "' data-toggle='tooltip' data-placement='top' title='View Results'>"
        + "<i class='fa fa-bar-chart'></i>"
        + "</a>";
    if (c.is_owner) {
        html += "<span data-toggle='modal' data-backdrop='static' data-target='#modal'>"
            + "<button class='btn btn-success' style='" + btnStyle + "' data-toggle='tooltip' data-placement='top' title='Copy Campaign' onclick='copy(" + idx + ")'>"
            + "<i class='fa fa-copy'></i>"
            + "</button></span>"
            + "<button class='btn btn-danger' style='" + btnStyle + "' onclick='deleteCampaign(" + idx + ")' data-toggle='tooltip' data-placement='top' title='Delete Campaign'>"
            + "<i class='fa fa-trash-o'></i>"
            + "</button>";
    } else {
        // viewer — disabled 로 표시. modal trigger span 도 제거(disabled 인데 modal 열리면 안 됨).
        html += "<button class='btn btn-success' style='" + btnStyle + "' data-toggle='tooltip' data-placement='top' title='공유받은 캠페인은 복제할 수 없습니다' disabled>"
            + "<i class='fa fa-copy'></i>"
            + "</button>"
            + "<button class='btn btn-danger' style='" + btnStyle + "' data-toggle='tooltip' data-placement='top' title='공유받은 캠페인은 삭제할 수 없습니다' disabled>"
            + "<i class='fa fa-trash-o'></i>"
            + "</button>";
    }
    html += "</div>";
    return html;
}

function buildRow(c, idx) {
    // (#66) Name 컬럼 아래 날짜는 launch_date (발송 예정/완료 시각) 표시.
    // 기존엔 created_date 였으나, Status hover tooltip 제거(중복)와 함께
    // 사용자에게 더 의미 있는 시각인 launch_date 로 일원화.
    var campaign_date = moment(c.launch_date).format('YYYY.MM.DD, HH:mm:ss');
    var label = (statuses[c.status] && statuses[c.status].label) || "label-default";
    var ownerNote = (!c.is_owner && c.owner_username)
        ? " <small class='text-muted'>— by " + escapeHtml(c.owner_username) + "</small>"
        : "";

    var nameCell = escapeHtml(c.name) + "<br>"
        + "<small class='text-muted'>" + escapeHtml(campaign_date) + "</small>"
        + ownerNote;

    return [
        nameCell,
        getStat(c, 'sent'),
        getStat(c, 'opened'),
        getStat(c, 'clicked'),
        getStat(c, 'submitted'),
        getStat(c, 'executed'),
        getStat(c, 'reported'),
        getStat(c, 'trained'),
        // (#66) Status hover tooltip 제거 — 같은 행에 모든 stat + launch_date
        // 이 표시되어 정보 중복이었다.
        "<span class=\"label " + label + "\">" + c.status + "</span>",
        buildActionsCell(c, idx),
        // (#66) hidden 컬럼 — DataTables 의 search hook 과 차트 동기화에서
        // 캠페인 id 를 안정적으로 읽기 위함. data[N] 으로 직접 접근.
        String(c.id)
    ];
}

/* ============================================================
 * 5) 차트 동기화 — 가시 행 기반으로 재집계
 * ============================================================ */

// DataTables 가시 행에서 캠페인 id 를 뽑아 차트 갱신.
// 필터(allowedCampaignIds) + DataTables 검색 모두 적용된 결과.
function refreshChartsFromVisibleRows() {
    if (!campaignTable) return;
    var visibleIds = {};
    campaignTable.rows({ search: 'applied' }).every(function () {
        // (#66) hidden 컬럼(인덱스 10) 에서 campaign id 직접 읽음.
        var cid = this.data()[10];
        if (cid) visibleIds[cid] = true;
    });
    var visible = campaigns.filter(function (c) { return visibleIds[String(c.id)]; });
    generateStatsPieCharts(visible);
    generateClickRateOverTimeChart(visible);
}

/* ============================================================
 * 6) 필터 — DB 영구 저장
 * ============================================================ */

// dashboard_campaign_filter (JSON 문자열) 를 파싱해 Set 또는 null 반환.
// "" 또는 "[]" → null (= 전체 표시)
function parseFilterJson(s) {
    if (!s) return null;
    try {
        var arr = JSON.parse(s);
        if (!Array.isArray(arr) || arr.length === 0) return null;
        var set = {};
        arr.forEach(function (id) { set[id] = true; });
        return set;
    } catch (e) {
        return null;
    }
}

// 현재 필터(allowedCampaignIds) 에 맞는 캠페인만 통과시키는 DataTables search 함수.
// table.draw() 호출 시 자동으로 행 가시성 결정.
function dashboardFilterFn(settings, data, dataIndex, rowData) {
    if (settings.nTable.id !== 'campaignTable') return true;
    if (!allowedCampaignIds) return true;  // null = 전체 표시
    // (#66) hidden 컬럼(인덱스 10) 에 campaign id 가 문자열로 저장돼 있음.
    var cid = data && data[10];
    if (!cid) return true;
    return !!allowedCampaignIds[cid];
}

// 필터를 DB 에서 로드하고 적용.
function loadFilterFromServer() {
    return api.preferences.get()
        .done(function (data) {
            allowedCampaignIds = parseFilterJson(data && data.dashboard_campaign_filter);
            if (campaignTable) {
                campaignTable.draw();
                refreshChartsFromVisibleRows();
            }
        })
        .fail(function () {
            // 환경설정 로드 실패는 치명적이지 않음 — 전체 표시로 진행.
            allowedCampaignIds = null;
        });
}

function saveFilterToServer(idsArr) {
    var body = { dashboard_campaign_filter: idsArr.length === 0 ? "" : JSON.stringify(idsArr) };
    return api.preferences.put(body);
}

/* ============================================================
 * 7) 캠페인 선택 모달 — 검색/세부정보 토글/적용
 * ============================================================ */

function openCampaignFilterModal() {
    // 모달 진입 시 현재 상태를 draft 로 복제 (취소 시 원복).
    filterDraftIds = {};
    if (allowedCampaignIds) {
        Object.keys(allowedCampaignIds).forEach(function (k) { filterDraftIds[k] = true; });
    } else {
        // 필터 미적용 상태 = "전체 선택"으로 시작
        campaigns.forEach(function (c) { filterDraftIds[c.id] = true; });
    }
    $('#campaignFilterSearch').val('');
    renderCampaignFilterList('');
    $('#campaignFilterModal').modal('show');
}

function renderCampaignFilterList(searchTerm) {
    var $list = $('#campaignFilterList');
    $list.empty();

    var term = (searchTerm || '').toLowerCase().trim();
    var items = campaigns.filter(function (c) {
        if (!term) return true;
        var hay = [
            c.name || '',
            c.status || '',
            c.owner_username || ''
        ].join(' ').toLowerCase();
        return hay.indexOf(term) !== -1;
    });

    if (items.length === 0) {
        $list.append('<p class="text-muted text-center" style="padding:20px;">검색 결과가 없습니다.</p>');
        updateFilterCount();
        return;
    }

    items.forEach(function (c) {
        var checked = filterDraftIds[c.id] ? 'checked' : '';
        var date = moment(c.created_date).format('YYYY-MM-DD HH:mm');
        var label = (statuses[c.status] && statuses[c.status].label) || 'label-default';
        var ownerNote = (!c.is_owner && c.owner_username)
            ? "<span class='text-muted'> · by " + escapeHtml(c.owner_username) + "</span>"
            : '';
        var total = (c.stats && c.stats.total) || 0;

        var $row = $(
            '<div class="campaign-filter-row" data-campaign-id="' + c.id + '" '
            + 'style="padding:8px 10px;border-bottom:1px solid #eee;">'
            +   '<div style="display:flex;align-items:flex-start;gap:8px;">'
            +     '<input type="checkbox" class="campaign-filter-cb" '
            +       'data-cid="' + c.id + '" ' + checked + ' style="margin-top:4px;">'
            +     '<div style="flex:1;">'
            +       '<div><strong>' + escapeHtml(c.name) + '</strong></div>'
            +       '<div class="text-muted" style="font-size:12px;margin-top:2px;">'
            +         escapeHtml(date)
            +         ' · <span class="label ' + label + '" style="font-size:11px;">' + escapeHtml(c.status) + '</span>'
            +         ' · 수신자 ' + total + '명'
            +         ownerNote
            +       '</div>'
            +     '</div>'
            +     '<button type="button" class="btn btn-info btn-xs campaign-filter-detail" '
            +       'data-cid="' + c.id + '">'
            +       '<i class="fa fa-caret-down"></i> 세부'
            +     '</button>'
            +   '</div>'
            +   '<div class="campaign-filter-detail-body" data-cid="' + c.id + '" '
            +     'style="display:none;margin-top:8px;margin-left:26px;padding:8px;background:#f7f7f7;border-radius:4px;font-size:12px;">'
            +   '</div>'
            + '</div>'
        );
        $list.append($row);
    });

    updateFilterCount();
}

function buildDetailBody(c) {
    // (#66) Go 의 time.Time 제로값("0001-01-01T00:00:00Z") 은 truthy 라
    // 단순 if-truthy 만으로는 "-" 로 치환되지 않는다. 연도가 1년이면 미설정 간주.
    var isZeroTime = function (s) { return !s || /^0001-01-01/.test(s); };
    var launchDate = isZeroTime(c.launch_date)   ? '-' : moment(c.launch_date).format('YYYY-MM-DD HH:mm');
    var sendBy     = isZeroTime(c.send_by_date)  ? '-' : moment(c.send_by_date).format('YYYY-MM-DD HH:mm');
    var total      = (c.stats && c.stats.total) || 0;

    var html = ''
        + '<div><strong>발송 시작:</strong> ' + escapeHtml(launchDate) + '</div>'
        + '<div><strong>발송 마감:</strong> ' + escapeHtml(sendBy)     + '</div>'
        + '<div style="margin-top:6px;">'
        + '  <strong>Stats:</strong> '
        + '  Recipients ' + total + ' · '
        + '  Sent ' + getStat(c, 'sent') + ' · '
        + '  Opened ' + getStat(c, 'opened') + ' · '
        + '  Clicked ' + getStat(c, 'clicked') + ' · '
        + '  Submitted ' + getStat(c, 'submitted') + ' · '
        + '  Executed ' + getStat(c, 'executed') + ' · '
        + '  Reported ' + getStat(c, 'reported') + ' · '
        + '  Trained ' + getStat(c, 'trained')
        + '</div>'
        + '<div style="margin-top:8px;">'
        + '  <a class="btn btn-primary btn-xs" href="/campaigns/' + c.id + '">'
        + '    <i class="fa fa-bar-chart"></i> View Results'
        + '  </a>'
        + '</div>';
    return html;
}

function updateFilterCount() {
    var sel = Object.keys(filterDraftIds || {}).length;
    var total = campaigns.length;
    $('#campaignFilterCount').text(sel + ' / ' + total + ' 선택됨');
}

/* ============================================================
 * 8) Toolbar — DataTables length 옆에 [+ New Campaign] [⚙ 캠페인 선택] 주입
 * ============================================================ */

function installToolbarButtons() {
    var $length = $('#campaignTable_wrapper .dataTables_length');
    if (!$length.length) return;
    if ($length.find('#dashboardToolbarContainer').length) return; // 중복 방지

    var $container = $('<div id="dashboardToolbarContainer" style="display:inline-block;margin-left:12px;vertical-align:middle;"></div>');
    $container.append(
        '<button type="button" class="btn btn-primary btn-sm" '
        + 'data-toggle="modal" data-backdrop="static" data-target="#modal" onclick="edit(\'new\')">'
        + '<i class="fa fa-plus"></i> New Campaign'
        + '</button>'
    );
    $container.append(
        '<button type="button" class="btn btn-info btn-sm" id="openCampaignFilterBtn" style="margin-left:6px;">'
        + '<i class="fa fa-filter"></i> 캠페인 선택'
        + '</button>'
    );
    $length.append($container);
}

/* ============================================================
 * 9) 캠페인 CRUD (campaigns.js 흡수)
 * ============================================================ */

// labels — campaigns.js 의 deleteCampaign / Swal 흐름과 호환용. campaigns.js 폐기로 여기에 둠.
var labels = {
    "In progress": "label-primary",
    "Queued": "label-info",
    "Completed": "label-success",
    "Emails Sent": "label-success",
    "Error": "label-danger"
};

function launch() {
    Swal.fire({
        title: "Are you sure?",
        text: "This will schedule the campaign to be launched.",
        type: "question",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Launch",
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        showLoaderOnConfirm: true,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                var groups = [];
                $("#users").select2("data").forEach(function (group) {
                    groups.push({ name: group.text });
                });
                var send_by_date = $("#send_by_date").val();
                if (send_by_date != "") {
                    send_by_date = moment(send_by_date, "MMMM Do YYYY, h:mm a").utc().format();
                }
                campaign = {
                    name: $("#name").val(),
                    template: { name: $("#template").select2("data")[0].text },
                    url: $("#url").val(),
                    page: { name: $("#page").select2("data")[0].text },
                    smtp: { name: $("#profile").select2("data")[0].text },
                    launch_date: moment($("#launch_date").val(), "MMMM Do YYYY, h:mm a").utc().format(),
                    send_by_date: send_by_date || null,
                    groups: groups
                };
                api.campaigns.post(campaign)
                    .success(function (data) {
                        resolve();
                        campaign = data;
                    })
                    .error(function (data) {
                        $("#modal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-danger\">"
                            + "<i class=\"fa fa-exclamation-circle\"></i> " + data.responseJSON.message + "</div>");
                        Swal.close();
                    });
            });
        }
    }).then(function (result) {
        if (result.value) {
            Swal.fire('Campaign Scheduled!', 'This campaign has been scheduled for launch!', 'success');
        }
        $('button:contains("OK")').on('click', function () {
            window.location = "/campaigns/" + campaign.id.toString();
        });
    });
}

function sendTestEmail() {
    var test_email_request = {
        template: { name: $("#template").select2("data")[0].text },
        name:       $("input[name=to_name]").val(),
        department: $("input[name=to_department]").val(),
        email:      $("input[name=to_email]").val(),
        position:   $("input[name=to_position]").val(),
        url:        $("#url").val(),
        page:       { name: $("#page").select2("data")[0].text },
        smtp:       { name: $("#profile").select2("data")[0].text }
    };
    var btnHtml = $("#sendTestModalSubmit").html();
    $("#sendTestModalSubmit").html('<i class="fa fa-spinner fa-spin"></i> Sending');
    api.send_test_email(test_email_request)
        .success(function () {
            $("#sendTestEmailModal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-success\">"
                + "<i class=\"fa fa-check-circle\"></i> Sent!</div>");
            $("#sendTestModalSubmit").html(btnHtml);
        })
        .error(function (data) {
            $("#sendTestEmailModal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-danger\">"
                + "<i class=\"fa fa-exclamation-circle\"></i> " + data.responseJSON.message + "</div>");
            $("#sendTestModalSubmit").html(btnHtml);
        });
}

function dismiss() {
    $("#modal\\.flashes").empty();
    $("#name").val("");
    $("#template").val("").change();
    $("#page").val("").change();
    $("#url").val("");
    $("#profile").val("").change();
    $("#users").val("").change();
    $("#modal").modal('hide');
}

function deleteCampaign(idx) {
    Swal.fire({
        title: "Are you sure?",
        text: "This will delete the campaign. This can't be undone!",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete " + campaigns[idx].name,
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                api.campaignId.delete(campaigns[idx].id)
                    .success(function () { resolve(); })
                    .error(function (data) { reject(data.responseJSON.message); });
            });
        }
    }).then(function (result) {
        if (result.value) {
            Swal.fire('Campaign Deleted!', 'This campaign has been deleted!', 'success');
        }
        $('button:contains("OK")').on('click', function () { location.reload(); });
    });
}

// (#66) 캠페인 생성 전제조건 검사. 위→아래 순으로 순차 검사하고, 첫 부족 항목이
// 발견되면 그 안내만 표시하고 이후 단계는 호출하지 않는다 (위 단계가 비어
// 있는데 아래 단계 안내를 함께 띄우면 사용자가 혼란).
// 검사 순서: Email Template → Landing Page → Sending Profile → Groups
//          (모달 폼의 위→아래 순과 일치).
function setupOptions() {
    resetSetupOptionsUI();
    checkTemplates();
}

// 모달 재오픈 시 이전 상태(숨김/비활성/안내) 초기화. dismiss() 가 input 값만
// 비우고 표시 상태는 안 건드리므로 여기서 명시적으로 복구.
function resetSetupOptionsUI() {
    $("#modal\\.flashes").empty();
    ["template", "page", "profile", "users"].forEach(function (id) {
        $("#" + id).show();
        $("#" + id).closest(".form-group, .input-group").find("label[for='" + id + "']").show();
    });
    // template/page/profile 은 form-group 직속이지만 profile 은 input-group 안에
    // Send Test Email 버튼과 함께 있어서 wrapper 도 같이 보여준다.
    $("#profile").closest(".input-group").show();
    $("#launchButton").prop("disabled", false);
}

// 부족 항목 안내 helper.
// label    : 모달 라벨 텍스트 (예: "Email Templates")
// linkPath : 이동할 어드민 페이지 (예: "/templates")
// hideId   : 숨길 입력 필드 id (예: "template")
function showSetupMissingAlert(label, linkPath, hideId) {
    $("#modal\\.flashes").empty().append(
        "<div style='text-align:center' class='alert alert-warning'>"
        + "<i class='fa fa-exclamation-circle'></i> "
        + "캠페인을 만들려면 먼저 " + escapeHtml(label) + " 을(를) 등록해야 합니다. "
        + "<a href='" + linkPath + "' class='btn btn-warning btn-xs' style='margin-left:8px;'>"
        + "<i class='fa fa-arrow-right'></i> 이동"
        + "</a>"
        + "</div>"
    );
    // 부족 항목부터 아래의 모든 입력 필드를 숨겨서 빈 박스가 자리 차지하는
    // 어색함 제거. 순서대로 hideId 이후 항목들을 모두 숨김.
    var order = ["template", "page", "profile", "users"];
    var startIdx = order.indexOf(hideId);
    if (startIdx >= 0) {
        order.slice(startIdx).forEach(function (id) {
            $("#" + id).hide();
            $("#" + id).closest(".form-group, .input-group").find("label[for='" + id + "']").hide();
            if (id === "profile") {
                $("#profile").closest(".input-group").hide();
            }
        });
    }
    $("#launchButton").prop("disabled", true);
}

function checkTemplates() {
    api.templates.get().success(function (templates) {
        if (templates.length == 0) {
            showSetupMissingAlert("Email Templates", "/templates", "template");
            return;
        }
        var template_s2 = $.map(templates, function (obj) { obj.text = obj.name; return obj; });
        var $sel = $("#template.form-control");
        $sel.select2({ placeholder: "Select a Template", data: template_s2 });
        if (templates.length === 1) { $sel.val(template_s2[0].id); $sel.trigger('change.select2'); }
        checkPages();
    });
}

function checkPages() {
    api.pages.get().success(function (pages) {
        if (pages.length == 0) {
            showSetupMissingAlert("Landing Pages", "/landing_pages", "page");
            return;
        }
        var page_s2 = $.map(pages, function (obj) { obj.text = obj.name; return obj; });
        var $sel = $("#page.form-control");
        $sel.select2({ placeholder: "Select a Landing Page", data: page_s2 });
        if (pages.length === 1) { $sel.val(page_s2[0].id); $sel.trigger('change.select2'); }
        checkProfiles();
    });
}

function checkProfiles() {
    api.SMTP.get().success(function (profiles) {
        if (profiles.length == 0) {
            showSetupMissingAlert("Sending Profiles", "/sending_profiles", "profile");
            return;
        }
        var profile_s2 = $.map(profiles, function (obj) { obj.text = obj.name; return obj; });
        var $sel = $("#profile.form-control");
        $sel.select2({ placeholder: "Select a Sending Profile", data: profile_s2 }).select2("val", profile_s2[0]);
        if (profiles.length === 1) { $sel.val(profile_s2[0].id); $sel.trigger('change.select2'); }
        checkGroups();
    });
}

function checkGroups() {
    api.groups.summary().success(function (summaries) {
        var groups = summaries.groups;
        if (groups.length == 0) {
            showSetupMissingAlert("Users &amp; Groups", "/groups", "users");
            return;
        }
        var group_s2 = $.map(groups, function (obj) {
            obj.text = obj.name;
            obj.title = obj.num_targets + " targets";
            return obj;
        });
        $("#users.form-control").select2({ placeholder: "Select Groups", data: group_s2 });
    });
}

function edit(_campaign) {
    setupOptions();
}

function copy(idx) {
    setupOptions();
    api.campaignId.get(campaigns[idx].id)
        .success(function (c) {
            $("#name").val("Copy of " + c.name);
            ["template", "page", "profile"].forEach(function (kind) {
                var obj = (kind === "profile") ? c.smtp : c[kind];
                if (!obj.id) {
                    $("#" + (kind === "profile" ? "profile" : kind)).val("").change();
                    $("#" + (kind === "profile" ? "profile" : kind)).select2({ placeholder: obj.name });
                } else {
                    $("#" + (kind === "profile" ? "profile" : kind)).val(obj.id.toString());
                    $("#" + (kind === "profile" ? "profile" : kind)).trigger("change.select2");
                }
            });
            $("#url").val(c.url);
        })
        .error(function (data) {
            $("#modal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-danger\">"
                + "<i class=\"fa fa-exclamation-circle\"></i> " + data.responseJSON.message + "</div>");
        });
}

/* ============================================================
 * 10) DOM ready
 * ============================================================ */

$(document).ready(function () {
    Highcharts.setOptions({ global: { useUTC: false } });

    // Modal stacking + datetimepicker — campaigns.js 에서 이전.
    $("#launch_date").datetimepicker({
        widgetPositioning: { vertical: "bottom" },
        showTodayButton: true,
        defaultDate: moment(),
        format: "MMMM Do YYYY, h:mm a"
    });
    $("#send_by_date").datetimepicker({
        widgetPositioning: { vertical: "bottom" },
        showTodayButton: true,
        useCurrent: false,
        format: "MMMM Do YYYY, h:mm a"
    });
    $('.modal').on('hidden.bs.modal', function () {
        $(this).removeClass('fv-modal-stack');
        $('body').data('fv_open_modals', $('body').data('fv_open_modals') - 1);
    });
    $('.modal').on('shown.bs.modal', function () {
        if (typeof $('body').data('fv_open_modals') == 'undefined') $('body').data('fv_open_modals', 0);
        if ($(this).hasClass('fv-modal-stack')) return;
        $(this).addClass('fv-modal-stack');
        $('body').data('fv_open_modals', $('body').data('fv_open_modals') + 1);
        $(this).css('z-index', 1040 + (10 * $('body').data('fv_open_modals')));
        $('.modal-backdrop').not('.fv-modal-stack').css('z-index', 1039 + (10 * $('body').data('fv_open_modals')));
        $('.modal-backdrop').not('fv-modal-stack').addClass('fv-modal-stack');
    });
    $(document).on('hidden.bs.modal', '.modal', function () {
        $('.modal:visible').length && $(document.body).addClass('modal-open');
    });
    $('#modal').on('hidden.bs.modal', function () { dismiss(); });

    // Select2 defaults — campaigns.js 에서 이전.
    $.fn.select2.defaults.set("width", "100%");
    $.fn.select2.defaults.set("dropdownParent", $("#modal_body"));
    $.fn.select2.defaults.set("theme", "bootstrap");
    $.fn.select2.defaults.set("sorter", function (data) {
        return data.sort(function (a, b) {
            if (a.text.toLowerCase() > b.text.toLowerCase()) return 1;
            if (a.text.toLowerCase() < b.text.toLowerCase()) return -1;
            return 0;
        });
    });

    // DataTables 의 search hook — 우리 dashboardFilterFn 을 등록.
    $.fn.dataTable.ext.search.push(dashboardFilterFn);

    // 캠페인 목록 + 필터 로드 병렬 실행.
    $.when(api.campaigns.summary(), api.preferences.get()).done(function (campaignsResp, prefsResp) {
        // jQuery ajax 의 $.when 다중 응답은 각 인자를 [data, statusText, xhr] 배열로 준다.
        var data  = campaignsResp[0];
        var prefs = prefsResp[0];

        $("#loading").hide();
        campaigns = (data && data.campaigns) || [];
        allowedCampaignIds = parseFilterJson(prefs && prefs.dashboard_campaign_filter);

        if (campaigns.length === 0) {
            $("#emptyMessage").show();
            return;
        }

        $("#dashboard").show();
        campaignTable = $("#campaignTable").DataTable({
            columnDefs: [
                { orderable: false, targets: "no-sort" },
                { className: "color-sent",     targets: [1] },
                { className: "color-opened",   targets: [2] },
                { className: "color-clicked",  targets: [3] },
                { className: "color-success",  targets: [4] },
                { className: "color-executed", targets: [5] },
                { className: "color-reported", targets: [6] },
                { className: "color-trained",  targets: [7] },
                // (#66) 캠페인 id hidden 컬럼 — filter / 차트 동기화 용.
                // searchable:true 유지 — false 로 두면 search hook 의 data 인자에서 빈 문자열로 들어와 cid 매칭이 깨진다.
                { visible: false, targets: [10] }
            ],
            order: [[0, "desc"]]
        });

        var rows = campaigns.map(function (c, i) { return buildRow(c, i); });
        campaignTable.rows.add(rows).draw();

        installToolbarButtons();
        $('[data-toggle="tooltip"]').tooltip();

        // 초기 차트 — 필터 적용된 가시 행 기준.
        refreshChartsFromVisibleRows();

        // DataTables 검색/페이지/draw 후 차트 동기화.
        campaignTable.on('search.dt draw.dt', function () {
            refreshChartsFromVisibleRows();
        });
    }).fail(function () {
        $("#loading").hide();
        errorFlash("Error fetching campaigns");
    });

    /* === 캠페인 필터 모달 핸들러 === */

    $(document).on('click', '#openCampaignFilterBtn', openCampaignFilterModal);

    $(document).on('input', '#campaignFilterSearch', function () {
        renderCampaignFilterList($(this).val());
    });

    $(document).on('click', '#campaignFilterCheckAll', function () {
        // 현재 검색 필터된 항목만 전체 선택.
        $('#campaignFilterList .campaign-filter-cb').each(function () {
            var cid = $(this).data('cid');
            filterDraftIds[cid] = true;
            this.checked = true;
        });
        updateFilterCount();
    });

    $(document).on('click', '#campaignFilterCheckNone', function () {
        $('#campaignFilterList .campaign-filter-cb').each(function () {
            var cid = $(this).data('cid');
            delete filterDraftIds[cid];
            this.checked = false;
        });
        updateFilterCount();
    });

    $(document).on('change', '.campaign-filter-cb', function () {
        var cid = $(this).data('cid');
        if (this.checked) filterDraftIds[cid] = true;
        else delete filterDraftIds[cid];
        updateFilterCount();
    });

    $(document).on('click', '.campaign-filter-detail', function () {
        var $btn  = $(this);
        var cid   = $btn.data('cid');
        var $body = $('.campaign-filter-detail-body[data-cid="' + cid + '"]');
        if ($body.is(':visible')) {
            $body.slideUp(120);
            $btn.html('<i class="fa fa-caret-down"></i> 세부');
        } else {
            // 본문 첫 펼침 시 빌드.
            if ($body.is(':empty')) {
                var c = campaigns.filter(function (x) { return x.id == cid; })[0];
                if (c) $body.html(buildDetailBody(c));
            }
            $body.slideDown(120);
            $btn.html('<i class="fa fa-caret-up"></i> 세부');
        }
    });

    $(document).on('click', '#campaignFilterApply', function () {
        // 빈 선택 = "전체 표시" 로 간주.
        var idsArr = Object.keys(filterDraftIds).map(function (k) { return parseInt(k, 10); });
        var isAll = (idsArr.length === campaigns.length);
        var saveArr = isAll ? [] : idsArr;
        saveFilterToServer(saveArr).done(function () {
            allowedCampaignIds = (saveArr.length === 0) ? null : (function () {
                var s = {};
                saveArr.forEach(function (id) { s[id] = true; });
                return s;
            })();
            $('#campaignFilterModal').modal('hide');
            if (campaignTable) {
                campaignTable.draw();
                // draw 이벤트가 자동으로 refreshChartsFromVisibleRows 호출.
            }
            successFlashFade('필터를 적용했습니다.', 3);
        }).fail(function (xhr) {
            errorFlash(extractErr(xhr));
        });
    });
});