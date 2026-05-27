var campaigns = []
// statuses is a helper map to point result statuses to ui classes
var statuses = {
    "Sent": {
        color: "#1abc9c",
        label: "label-success",
        icon:  "fa-envelope",
        point: "ct-point-sent"
    },
    "In progress": {
        label: "label-primary"
    },
    "Queued": {
        label: "label-info"
    },
    "Completed": {
        label: "label-success"
    },
    "Opened": {
        color: "#f9bf3b",
        label: "label-warning",
        icon:  "fa-envelope",
        point: "ct-point-opened"
    },
    "Reported": {
        color: "#45d6ef",
        label: "label-warning",
        icon:  "fa-bullhorn",
        point: "ct-point-reported"
    },
    "Clicked": {
        color: "#F39C12",
        label: "label-clicked",
        icon:  "fa-mouse-pointer",
        point: "ct-point-clicked"
    },
    "Success": {
        color: "#f05b4f",
        label: "label-danger",
        icon:  "fa-exclamation",
        point: "ct-point-clicked"
    },
    "Error": {
        color: "#6c7a89",
        label: "label-default",
        icon:  "fa-times",
        point: "ct-point-error"
    },
    "Error Sending Email": {
        color: "#6c7a89",
        label: "label-default",
        icon:  "fa-times",
        point: "ct-point-error"
    },
    "Submitted": {
        color: "#f05b4f",
        label: "label-danger",
        icon:  "fa-exclamation",
        point: "ct-point-clicked"
    },
    "Unknown": {
        color: "#6c7a89",
        label: "label-default",
        icon:  "fa-question",
        point: "ct-point-error"
    },
    "Sending": {
        color: "#428bca",
        label: "label-primary",
        icon:  "fa-spinner",
        point: "ct-point-sending"
    },
    "Campaign Created": {
        label: "label-success",
        icon:  "fa-rocket"
    },
    "Executed": {
        color: "#ff0000",
        label: "label-danger",
        icon:  "fa-exclamation-triangle",
        point: "ct-point-executed"
    },
    "Trained": {
        color: "#2727dd",
        label: "label-info",
        icon:  "fa-graduation-cap",
        point: "ct-point-trained"
    }
}

var statsMapping = {
    "sent":      "Sent",
    "opened":    "Opened",
    "reported":  "Reported",
    "clicked":   "Clicked",
    "submitted": "Submitted",
    "executed":  "Executed",
    "trained":   "Trained"
}

function deleteCampaign(idx) {
    if (confirm("Delete " + campaigns[idx].name + "?")) {
        api.campaignId.delete(campaigns[idx].id)
            .success(function (data) {
                successFlash(data.message)
                location.reload()
            })
    }
}

// (#47) 도넛 차트 cell 폭에 따라 폰트 크기 단계 결정.
// title = 차트 위 라벨 (Sent/Opened/...), center = 도넛 가운데 숫자.
function pickFontSizes(w) {
    if (w >= 160) return { title: 16, center: 16 };
    if (w >= 120) return { title: 14, center: 14 };
    if (w >= 90)  return { title: 12, center: 12 };
    return { title: 10, center: 10 };
}

/* Renders a pie chart using the provided chartops */
function renderPieChart(chartopts) {
    // 컨테이너 없으면 렌더 스킵
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
                    // (#47) 차트 타이틀 (Sent/Opened/...) 도 cell 크기에 맞춰 축소
                    chart.title && chart.title.css({ fontSize: sizes.title + 'px' });
                    this.innerText = rend.text(chartopts['data'][0].count, left, top).
                    attr({
                        'text-anchor': 'middle',
                        'dominant-baseline': 'central',
                        'font-size': sizes.center + 'px',
                        'font-weight': 'bold',
                        'fill': chartopts['colors'][0],
                        'font-family': 'Helvetica,Arial,sans-serif'
                    }).add()
                },
                render: function () {
                    // (#45) 창 리사이즈 시 pie.center 가 변하므로 좌표 재계산.
                    // (#47) 리사이즈/Refresh 후 cell 폭에 맞춰 폰트도 함께 축소.
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
        title: {
            text: chartopts['title']
        },
        plotOptions: {
            pie: {
                innerSize: '80%',
                dataLabels: {
                    enabled: false
                }
            }
        },
        credits: {
            enabled: false
        },
        tooltip: {
            formatter: function () {
                if (this.key == undefined) {
                    return false
                }
                return '<span style="color:' + this.color + '">\u25CF</span>' + this.point.name + ': <b>' + this.y + '%</b><br/>'
            }
        },
        series: [{
            data: chartopts['data'],
            colors: chartopts['colors'],
        }]
    })
}

/** 안전 숫자 게터 */
function getStat(campaign, key) {
    var v = campaign && campaign.stats ? campaign.stats[key] : undefined;
    return (typeof v === 'number' && isFinite(v)) ? v : 0;
}

function generateStatsPieCharts(campaigns) {
    // 1) 모든 지표를 0으로 초기화 (항상 렌더: 0이어도 도넛이 보이게)
    var buckets = {
        sent: 0,
        opened: 0,
        clicked: 0,
        submitted: 0,
        executed: 0,
        reported: 0,
        trained: 0
    };
    var total = 0;

    // 2) 합산
    $.each(campaigns, function (i, campaign) {
        var stats = campaign.stats || {};
        total += (typeof stats.total === 'number' && isFinite(stats.total)) ? stats.total : 0;

        // 존재/미존재와 상관없이 안전 합산
        buckets.sent      += getStat(campaign, 'sent');
        buckets.opened    += getStat(campaign, 'opened');
        buckets.clicked   += getStat(campaign, 'clicked');
        buckets.submitted += getStat(campaign, 'submitted');
        buckets.executed  += getStat(campaign, 'executed');
        buckets.reported  += getStat(campaign, 'reported');
        buckets.trained   += getStat(campaign, 'trained');
    });

    // 3) 항상 모든 키에 대해 차트 렌더 (0이어도)
    Object.keys(statsMapping).forEach(function (statusKey) {
        var status_label = statsMapping[statusKey];
        var count = buckets[statusKey] || 0;
        var denom = total || 1; // total=0이면 NaN 방지
        // 소수점 1자리 정밀도 유지 (예: 16.5%). tooltip 의 Highcharts.numberFormat 과 일치.
        var pct = Math.round((count / denom) * 1000) / 10;

        var stats_data = [
            { name: status_label, y: pct, count: count },
            { name: '', y: 100 - pct }
        ];

        renderPieChart({
            elemId: statusKey + '_chart',
            title: status_label,
            name: statusKey,
            data: stats_data,
            colors: [ (statuses[status_label] && statuses[status_label].color) || '#95a5a6', '#dddddd' ]
        });
    });
}

// 분(min) 단위 숫자를 "Xh Ym" 또는 "Ym" 포맷으로 변환. X축 라벨 + tooltip 공용.
function formatElapsed(totalMin) {
    var mins = Math.floor(totalMin);
    if (mins < 0) mins = 0;
    var h = Math.floor(mins / 60);
    var m = mins % 60;
    if (h <= 0) return m + 'm';
    return h + 'h ' + m + 'm';
}

// (#49) Dashboard "Click Rate Over Time" — 캠페인별 누적 클릭률 라인.
// X축 = 발송 시작(launch_date) 후 경과 시간(분), Y축 = 누적 distinct 클릭자 / total × 100.
// 최근 10개 캠페인만. summary 응답의 click_timeline 을 사용 (백엔드에서 채움).
function generateClickRateOverTimeChart(campaigns) {
    if (!document.getElementById('click_rate_over_time_chart')) return;

    // 최근 10개 (created_date 기준 내림차순) 선별
    var recent = campaigns.slice().sort(function (a, b) {
        return new Date(b.created_date) - new Date(a.created_date);
    }).slice(0, 10);

    var series = [];
    $.each(recent, function (i, campaign) {
        var timeline = campaign.click_timeline || [];
        var total = (campaign.stats && typeof campaign.stats.total === 'number') ? campaign.stats.total : 0;
        if (timeline.length === 0 || total === 0) {
            // 데이터 없거나 발송 0 — 라인 그릴 게 없음 (legend 에도 안 나오게 skip)
            return true;
        }
        // launch_date 기준 경과시간 (분) 계산
        var launchMs = new Date(campaign.launch_date).getTime();
        var points = [];
        // x=0 시점에 0% 시작점 강제 추가 (라인 시작이 갑자기 점프하지 않도록).
        // launch 시각을 원본 시각으로 함께 저장해 tooltip 에서 표시.
        points.push({ x: 0, y: 0, original_time: campaign.launch_date });
        $.each(timeline, function (j, pt) {
            var elapsedMin = (new Date(pt.time).getTime() - launchMs) / 60000;
            if (elapsedMin < 0) elapsedMin = 0; // launch 전 클릭 (시계 어긋남) 방어
            var rate = (pt.count / total) * 100;
            points.push({
                x: elapsedMin,
                y: Math.round(rate * 100) / 100,
                original_time: pt.time
            });
        });
        series.push({
            name: campaign.name,
            data: points
        });
    });

    Highcharts.chart('click_rate_over_time_chart', {
        chart: {
            type: 'line',
            zoomType: 'x'
        },
        title: {
            text: 'Click Rate Over Time (Recent ' + series.length + ' Campaigns)'
        },
        xAxis: {
            title: { text: 'Elapsed time since launch' },
            min: 0,
            labels: {
                formatter: function () {
                    return formatElapsed(this.value);
                }
            }
        },
        yAxis: {
            min: 0,
            ceiling: 100, // 자동 max 가 100 을 절대 넘지 않음 (누적 클릭률 상한)
            softMax: 10,  // 데이터 max 가 너무 작아도 최소 0~10 범위 보장
            title: { text: 'Cumulative Click Rate (%)' }
        },
        tooltip: {
            shared: false,
            formatter: function () {
                // point.original_time 은 series 빌드 시 점 객체에 함께 저장된
                // UTC ISO 시각. moment 로 로컬 변환해 표시.
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
                // 숨김 상태 = 회색, 표시 상태 = series color.
                // Highcharts 가 토글 후 자동 재호출하지 않으므로
                // events.legendItemClick 에서 강제 재렌더 (아래 plotOptions 참조).
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
                    // Highcharts 의 기본 legendItemClick 흐름이 끝난 다음 틱에
                    // legend 를 강제 재렌더해 labelFormatter 가 갱신된 visible
                    // 상태로 다시 호출되도록 함. 인라인 color 가 stale 로 남는 결함 차단.
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

function generateTimelineChart(campaigns) {
    var overview_data = []
    $.each(campaigns, function (i, campaign) {
        var campaign_date = moment.utc(campaign.created_date).local()
        // Add it to the chart data
        campaign.y = 0
        // Clicked events also contain our data submitted events
        campaign.y += getStat(campaign, 'clicked')
        var denom = (campaign.stats && typeof campaign.stats.total === 'number' && isFinite(campaign.stats.total))
            ? campaign.stats.total : 0;
        campaign.y = Math.floor((denom ? (campaign.y / denom) : 0) * 100)
        // Add the data to the overview chart
        overview_data.push({
            campaign_id: campaign.id,
            name: campaign.name,
            x: campaign_date.valueOf(),
            y: campaign.y
        })
    })
    Highcharts.chart('overview_chart', {
        chart: {
            zoomType: 'x',
            type: 'areaspline'
        },
        title: {
            text: 'Click Rate by Campaign'
        },
        xAxis: {
            type: 'datetime',
            dateTimeLabelFormats: {
                second: '%l:%M:%S',
                minute: '%l:%M',
                hour: '%l:%M',
                day: '%b %d, %Y',
                week: '%b %d, %Y',
                month: '%b %Y'
            }
        },
        yAxis: {
            min: 0,
            max: 100,
            title: {
                text: "Click Rate (%)"
            }
        },
        tooltip: {
            formatter: function () {
                return Highcharts.dateFormat('%A, %b %d %l:%M:%S %P', new Date(this.x)) +
                    '<br>' + this.point.name + '<br>Click Rate: <b>' + this.y + '%</b>'
            }
        },
        legend: {
            enabled: false
        },
        plotOptions: {
                series: {
                marker: {
                    enabled: true,
                    symbol: 'circle',
                    radius: 3
                },
                cursor: 'pointer',
                point: {
                    events: {
                        click: function (e) {
                            window.location.href = "/campaigns/" + this.campaign_id
                        }
                    }
                }
            }
        },
        credits: {
            enabled: false
        },
        series: [{
            data: overview_data,
            color: "#f05b4f",
            fillOpacity: 0.5
        }]
    })
}

$(document).ready(function () {
    Highcharts.setOptions({
        global: {
            useUTC: false
        }
    })
    api.campaigns.summary()
        .success(function (data) {
            $("#loading").hide()
            campaigns = data.campaigns
            if (campaigns.length > 0) {
                $("#dashboard").show()
                // Create the overview chart data
                campaignTable = $("#campaignTable").DataTable({
                    columnDefs: [{
                            orderable: false,
                            targets: "no-sort"
                        },
                        { className: "color-sent",     targets: [1] },
                        { className: "color-opened",   targets: [2] },
                        { className: "color-clicked",  targets: [3] },
                        { className: "color-success",  targets: [4] },
                        { className: "color-executed", targets: [5] },
                        { className: "color-reported", targets: [6] },
                        { className: "color-trained",  targets: [7] }
                    ],
                    order: [[1, "desc"]]
                });
                campaignRows = []
                $.each(campaigns, function (i, campaign) {
                    //var campaign_date = moment(campaign.created_date).format('MMMM Do YYYY, h:mm:ss a')
                    var campaign_date = moment(campaign.created_date).format('YYYY.MM.DD, HH:MM:SS')
                    var label = (statuses[campaign.status] && statuses[campaign.status].label) || "label-default";

                    //section for tooltips on the status of a campaign to show some quick stats
                    var launchDate;
                    var isFuture = moment(campaign.launch_date).isAfter(moment());
                    if (isFuture) {
                        launchDate = "Scheduled to start: " + moment(campaign.launch_date).format('MMMM Do YYYY, h:mm:ss a')
                        var quickStats = launchDate + "<br><br>" + "Number of recipients: " + ((campaign.stats && campaign.stats.total) || 0)
                    } else {
                        launchDate = "Launch Date: " + moment(campaign.launch_date).format('MMMM Do YYYY, h:mm:ss a')
                        var quickStats = launchDate
                            + "<br><br>Number of recipients: " + ((campaign.stats && campaign.stats.total) || 0)
                            + "<br><br>Opened: " + getStat(campaign,'opened')
                            + "<br><br>Clicked: " + getStat(campaign,'clicked')
                            + "<br><br>Submitted: " + getStat(campaign,'submitted')
                            + "<br><br>Executed: " + getStat(campaign,'executed')
                            + "<br><br>Errors: " + ((campaign.stats && campaign.stats.error) || 0)
                            + "<br><br>Reported: " + getStat(campaign,'reported')
                            + "<br><br>Trained: " + getStat(campaign,'trained')
                    }

                    campaignRows.push([
                        escapeHtml(campaign.name) + "<br>" + escapeHtml(campaign_date),
                        getStat(campaign,'sent'),
                        getStat(campaign,'opened'),
                        getStat(campaign,'clicked'),
                        getStat(campaign,'submitted'),
                        getStat(campaign,'executed'),
                        getStat(campaign,'reported'),
                        getStat(campaign,'trained'),
                        "<span class=\"label " + label + "\" data-toggle=\"tooltip\" data-placement=\"right\" data-html=\"true\" title=\"" + quickStats + "\">" + campaign.status + "</span>",
                        "<div class='pull-left'><a class='btn btn-primary' href='/campaigns/" + campaign.id + "' data-toggle='tooltip' data-placement='left' title='View Results'>\
                    <i class='fa fa-bar-chart'></i>\
                    </a>\
                    <button class='btn btn-danger' onclick='deleteCampaign(" + i + ")' data-toggle='tooltip' data-placement='left' title='Delete Campaign'>\
                    <i class='fa fa-trash-o'></i>\
                    </button></div>"
                    ])
                    $('[data-toggle="tooltip"]').tooltip()
                })
                campaignTable.rows.add(campaignRows).draw()

                generateStatsPieCharts(campaigns)
                generateClickRateOverTimeChart(campaigns)
            } else {
                $("#emptyMessage").show()
            }
        })
        .error(function () {
            errorFlash("Error fetching campaigns")
        })
})

