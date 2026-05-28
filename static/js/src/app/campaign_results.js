var map = null
var doPoll = true;

/* =========================
 * Status metadata + helpers
 * ========================= */

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
        icon:  "fa-envelope-open",
        point: "ct-point-opened"
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
    "Reported": {
        color: "#45d6ef",
        label: "label-info",
        icon:  "fa-bullhorn",
        point: "ct-point-reported"
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
    "Retrying": {
        color: "#6c7a89",
        label: "label-default",
        icon:  "fa-clock-o",
        point: "ct-point-error"
    },
    "Scheduled": {
        color: "#428bca",
        label: "label-primary",
        icon:  "fa-clock-o",
        point: "ct-point-sending"
    },
    "Campaign Created": {
        label: "label-success",
        icon:  "fa-rocket"
    },
    "Executed": {
        color: "#e74c3c",
        label: "label-danger",
        icon:  "fa-exclamation-triangle",
        point: "ct-point-executed"
    },
    "Trained": {
        color: "#2727dd",
        label: "label-info",
        icon:  "fa-graduation-cap",
        point: "ct-point-trained"
    },
    // Safe default when we don't recognize a status key
    "__default": {
        color: "#95a5a6",
        label: "label-default",
        icon:  "fa-circle",
        point: "ct-point-default"
    }
};

// Normalize server-provided status strings to our canonical keys
function normalizeStatus(raw) {
    if (!raw) return "";
    var s = ("" + raw).trim();
    switch (s) {
        // Sentinel 라벨로 정규화 (마이그레이션 전 방어)
        case "Email Sent":     return "Sent";
        case "Email Opened":   return "Opened";
        case "Clicked Link":   return "Clicked";
        case "Submitted Data": return "Submitted";
        case "Email Reported": return "Reported";
        // 기존 호환
        case "Success":        return "Submitted";
        case "Attach Opened":
        case "Attachment Opened":
        case "Attachment Executed":
            return "Executed";
        default:
            return s;
    }
}

// Safe metadata accessor
function getStatusMeta(name) {
    var key = normalizeStatus(name);
    return statuses[key] || statuses["__default"];
}

var statusMapping = {
    "Sent":      "sent",
    "Opened":    "opened",
    "Clicked":   "clicked",
    "Submitted": "submitted",
    "Reported":  "reported",
    "Executed":  "executed",
    "Trained":   "trained"
}

// This is an underwhelming attempt at an enum
// until I have time to refactor this appropriately.
var progressListing = [
    "Sent",
    "Opened",
    "Clicked",
    "Submitted"
]

var campaign = {}
var bubbles = []
var videoProgressTable = null

function dismiss() {
    $("#modal\\.flashes").empty()
    $("#modal").modal('hide')
    $("#resultsTable").dataTable().DataTable().clear().draw()
}

// Deletes a campaign after prompting the user
function deleteCampaign() {
    Swal.fire({
        title: "Are you sure?",
        text: "This will delete the campaign. This can't be undone!",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete Campaign",
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        showLoaderOnConfirm: true,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                api.campaignId.delete(campaign.id)
                    .success(function (msg) {
                        resolve()
                    })
                    .error(function (data) {
                        reject(data.responseJSON.message)
                    })
            })
        }
    }).then(function (result) {
        if(result.value){
            Swal.fire(
                'Campaign Deleted!',
                'This campaign has been deleted!',
                'success'
            );
        }
        $('button:contains("OK")').on('click', function () {
            location.href = '/campaigns'
        })
    })
}

// Completes a campaign after prompting the user
function completeCampaign() {
    Swal.fire({
        title: "Are you sure?",
        text: "Sentinel will stop processing events for this campaign",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Complete Campaign",
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        showLoaderOnConfirm: true,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                api.campaignId.complete(campaign.id)
                    .success(function (msg) {
                        resolve()
                    })
                    .error(function (data) {
                        reject(data.responseJSON.message)
                    })
            })
        }
    }).then(function (result) {
        if (result.value){
            Swal.fire(
                'Campaign Completed!',
                'This campaign has been completed!',
                'success'
            );
            $('#complete_button')[0].disabled = true;
            $('#complete_button').text('Completed!')
            doPoll = false;
        }
    })
}

// Exports campaign results as a CSV file
function exportAsCSV(scope) {
    exportHTML = $("#exportButton").html()

    // ▼ 새 분기: Events (CSV) — 첨부 Events.csv 포맷을 프런트에서 생성
    if (scope === "events_flat") {
        var payload = buildEventsFlatCSVData(campaign); // {fields, data}
        $("#exportButton").html('<i class="fa fa-spinner fa-spin"></i>')
        var csvString = Papa.unparse(payload, { 'escapeFormulae': true })
        var bom = new Uint8Array([0xEF, 0xBB, 0xBF])
        var csvData = new Blob([bom, csvString], { type: 'text/csv;charset=utf-8;' });
        var filename = (campaign.name || 'Campaign') + ' - Events.csv'
        if (navigator.msSaveBlob) {
            navigator.msSaveBlob(csvData, filename);
        } else {
            var csvURL = window.URL.createObjectURL(csvData);
            var dlLink = document.createElement('a');
            dlLink.href = csvURL;
            dlLink.setAttribute('download', filename)
            document.body.appendChild(dlLink)
            dlLink.click();
            document.body.removeChild(dlLink)
        }
        $("#exportButton").html(exportHTML)
        return;
    }

    var csvScope = null
    var filename = campaign.name + ' - ' + capitalize(scope) + '.csv'
    switch (scope) {
        case "results":
            csvScope = campaign.results
            break;
        case "events":
            csvScope = campaign.timeline
            break;
    }
    if (!csvScope) {
        return
    }
    $("#exportButton").html('<i class="fa fa-spinner fa-spin"></i>')
    var csvString = Papa.unparse(csvScope, {
        'escapeFormulae': true
    })
    var bom = new Uint8Array([0xEF, 0xBB, 0xBF])
    var csvData = new Blob([bom, csvString], {
        type: 'text/csv;charset=utf-8;'
    });
    if (navigator.msSaveBlob) {
        navigator.msSaveBlob(csvData, filename);
    } else {
        var csvURL = window.URL.createObjectURL(csvData);
        var dlLink = document.createElement('a');
        dlLink.href = csvURL;
        dlLink.setAttribute('download', filename)
        document.body.appendChild(dlLink)
        dlLink.click();
        document.body.removeChild(dlLink)
    }
    $("#exportButton").html(exportHTML)
}

function replay(event_idx) {
    request = campaign.timeline[event_idx]
    details = JSON.parse(request.details)
    url = null
    form = $('<form>').attr({
        method: 'POST',
        target: '_blank',
    })
    /* Create a form object and submit it */
    $.each(Object.keys(details.payload), function (i, param) {
        if (param == "rid") {
            return true;
        }
        if (param == "__original_url") {
            url = details.payload[param];
            return true;
        }
        $('<input>').attr({
            name: param,
        }).val(details.payload[param]).appendTo(form);
    })
    /* Ensure we know where to send the user */
    // Prompt for the URL
    Swal.fire({
        title: 'Where do you want the credentials submitted to?',
        input: 'text',
        showCancelButton: true,
        inputPlaceholder: "http://example.com/login",
        inputValue: url || "",
        inputValidator: function (value) {
            return new Promise(function (resolve, reject) {
                if (value) {
                    resolve();
                } else {
                    reject('Invalid URL.');
                }
            });
        }
    }).then(function (result) {
        if (result.value){
            url = result.value
            submitForm()
        }
    })
    return
    submitForm()

    function submitForm() {
        form.attr({
            action: url
        })
        form.appendTo('body').submit().remove()
    }
}

/**
 * Returns an HTML string that displays the OS and browser that clicked the link
 * or submitted credentials.
 *
 * @param {object} event_details - The "details" parameter for a campaign
 *  timeline event
 *
 */
var renderDevice = function (event_details) {
    var ua = UAParser(
        event_details && event_details.browser
            ? event_details.browser['user-agent'] || ''
            : ''
    )
    var detailsString = '<div class="timeline-device-details">'

    var deviceIcon = 'laptop'
    if (ua.device.type) {
        if (ua.device.type == 'tablet' || ua.device.type == 'mobile') {
            deviceIcon = ua.device.type
        }
    }

    var deviceVendor = ''
    if (ua.device.vendor) {
        deviceVendor = ua.device.vendor.toLowerCase()
        if (deviceVendor == 'microsoft') deviceVendor = 'windows'
    }

    var deviceName = 'Unknown'
    if (ua.os.name) {
        deviceName = ua.os.name
        if (deviceName == "Mac OS") {
            deviceVendor = 'apple'
        } else if (deviceName == "Windows") {
            deviceVendor = 'windows'
        }
        if (ua.device.vendor && ua.device.model) {
            deviceName = ua.device.vendor + ' ' + ua.device.model
        }
    }

    if (ua.os.version) {
        deviceName = deviceName + ' (OS Version: ' + ua.os.version + ')'
    }

    deviceString = '<div class="timeline-device-os"><span class="fa fa-stack">' +
        '<i class="fa fa-' + escapeHtml(deviceIcon) + ' fa-stack-2x"></i>' +
        '<i class="fa fa-vendor-icon fa-' + escapeHtml(deviceVendor) + ' fa-stack-1x"></i>' +
        '</span> ' + escapeHtml(deviceName) + '</div>'

    detailsString += deviceString

    var deviceBrowser = 'Unknown'
    var browserIcon = 'info-circle'
    var browserVersion = ''

    if (ua.browser && ua.browser.name) {
        deviceBrowser = ua.browser.name
        // Handle the "mobile safari" case
        deviceBrowser = deviceBrowser.replace('Mobile ', '')
        if (deviceBrowser) {
            browserIcon = deviceBrowser.toLowerCase()
            if (browserIcon == 'ie') browserIcon = 'internet-explorer'
        }
        browserVersion = '(Version: ' + ua.browser.version + ')'
    }

    var browserString = '<div class="timeline-device-browser"><span class="fa fa-stack">' +
        '<i class="fa fa-' + escapeHtml(browserIcon) + ' fa-stack-1x"></i></span> ' +
        deviceBrowser + ' ' + browserVersion + '</div>'

    detailsString += browserString
    detailsString += '</div>'
    return detailsString
}

function renderTimeline(data) {
    // executed는 테이블 데이터에 포함되지 않으므로 제거
    record = {
        "id": data[0],
        "name": data[2],
        "department": data[3],
        "email": data[4],
        "position": data[5],
        "status": data[6],
        "reported": data[12],
        "send_date": data[14]
    }
    results = '<div class="timeline col-sm-12 well well-lg">' +
        '<h6>Timeline for ' + escapeHtml(record.name) + ' ' + escapeHtml(record.department) +
        '</h6><span class="subtitle">Email: ' + escapeHtml(record.email) +
        '<br>Result ID: ' + escapeHtml(record.id) + '</span>' +
        '<div class="timeline-graph col-sm-6">'
    $.each(campaign.timeline, function (i, event) {
        if (!event.email || event.email == record.email) {
            // Add the event
            results += '<div class="timeline-entry">' +
                '    <div class="timeline-bar"></div>'
            var meta = getStatusMeta(event.message)
            results +=
                '    <div class="timeline-icon" style="background-color:' + (meta.color || '#95a5a6') + '">' +
                '    <i class="fa ' + meta.icon + '"></i></div>' +
                '    <div class="timeline-message">' + escapeHtml(event.message) +
                '    <span class="timeline-date">' + moment.utc(event.time).local().format('MMMM Do YYYY h:mm:ss a') + '</span>'
            if (event.details) {
                details = JSON.parse(event.details)
                if (event.message == "Clicked" || event.message == "Submitted") {
                    deviceView = renderDevice(details)
                    if (deviceView) {
                        results += deviceView
                    }
                }
                if (event.message == "Submitted") {
                    results += '<div class="timeline-replay-button"><button onclick="replay(' + i + ')" class="btn btn-success">'
                    results += '<i class="fa fa-refresh"></i> Replay Credentials</button></div>'
                    results += '<div class="timeline-event-details"><i class="fa fa-caret-right"></i> View Details</div>'
                }
                if (event.message == "Reported") {
                    results += '<div class="timeline-event-details"><i class="fa fa-caret-right"></i> View Details</div>'
                }
                if (event.message == "Trained") {
                    results += '<div class="timeline-event-details"><i class="fa fa-caret-right"></i> View Details</div>'
                }
                if (details.payload) {
                    results += '<div class="timeline-event-results">'
                    results += '    <table class="table table-condensed table-bordered table-striped">'
                    results += '        <thead><tr><th>Parameter</th><th>Value(s)</tr></thead><tbody>'
                    $.each(Object.keys(details.payload), function (i, param) {
                        if (param == "rid") {
                            return true;
                        }
                        var val = details.payload[param]
                        if (Array.isArray(val)) {
                            val = val.join(", ")
                        }
                        results += '    <tr>'
                        results += '        <td>' + escapeHtml(param) + '</td>'
                        results += '        <td>' + escapeHtml(val) + '</td>'
                        results += '    </tr>'
                    })
                    results += '       </tbody></table>'
                    results += '</div>'
                }
                if (details.error) {
                    results += '<div class="timeline-event-details"><i class="fa fa-caret-right"></i> View Details</div>'
                    results += '<div class="timeline-event-results">'
                    results += '<span class="label label-default">Error</span> ' + details.error
                    results += '</div>'
                }
            }
            results += '</div></div>'
        }
    })
    // Add the scheduled send event at the bottom
    if (record.status == "Scheduled" || record.status == "Retrying") {
        var meta2 = getStatusMeta(record.status)
        results += '<div class="timeline-entry">' +
            '    <div class="timeline-bar"></div>'
        results +=
            '    <div class="timeline-icon ' + meta2.label + '">' +
            '    <i class="fa ' + meta2.icon + '"></i></div>' +
            '    <div class="timeline-message">' + "Scheduled to send at " + record.send_date + '</span>'
    }
    results += '</div></div>'
    return results
}

var renderTimelineChart = function (chartopts) {
    return Highcharts.chart('timeline_chart', {
        chart: {
            zoomType: 'x',
            type: 'line',
            height: "200px"
        },
        title: {
            text: 'Campaign Timeline'
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
            max: 2,
            visible: false,
            tickInterval: 1,
            labels: {
                enabled: false
            },
            title: {
                text: ""
            }
        },
        tooltip: {
            formatter: function () {
                return Highcharts.dateFormat('%A, %b %d %l:%M:%S %P', new Date(this.x)) +
                    '<br>Event: ' + this.point.message + '<br>Email: <b>' + this.point.email + '</b>'
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
            },
            line: {
                states: {
                    hover: {
                        lineWidth: 1
                    }
                }
            }
        },
        credits: {
            enabled: false
        },
        series: [{
            data: chartopts['data'],
            dashStyle: "shortdash",
            color: "#cccccc",
            lineWidth: 1,
            turboThreshold: 0
        }]
    })
}

// (#47) 도넛 차트 cell 폭에 따라 폰트 크기 단계 결정.
// title = 차트 위 라벨 (Sent/Opened/...), center = 도넛 가운데 숫자 (캠페인 상세는 24까지).
function pickFontSizes(w) {
    if (w >= 160) return { title: 16, center: 24 };
    if (w >= 120) return { title: 14, center: 20 };
    if (w >= 90)  return { title: 12, center: 16 };
    return { title: 10, center: 12 };
}

/* Renders a pie chart using the provided chartops */
var renderPieChart = function (chartopts) {
    // 컨테이너가 없으면 렌더 스킵(에러 방지)
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
                    }).add();
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

/* Updates the bubbles on the map

@param {campaign.result[]} results - The campaign results to process
*/
var updateMap = function (results) {
    if (!map) {
        return
    }
    bubbles = []
    $.each(campaign.results, function (i, result) {
        // Check that it wasn't an internal IP
        if (result.latitude == 0 && result.longitude == 0) {
            return true;
        }
        newIP = true
        $.each(bubbles, function (i, bubble) {
            if (bubble.ip == result.ip) {
                bubbles[i].radius += 1
                newIP = false
                return false
            }
        })
        if (newIP) {
            bubbles.push({
                latitude: result.latitude,
                longitude: result.longitude,
                name: result.ip,
                fillKey: "point",
                radius: 2
            })
        }
    })
    map.bubbles(bubbles)
}

/**
 * Creates a status label for use in the results datatable
 * @param {string} status
 * @param {moment(datetime)} send_date
 */
function createStatusLabel(status, send_date) {
    var meta = getStatusMeta(status);
    var label = meta.label || "label-default";
    var statusColumn = "<span class=\"label " + label + "\">" + normalizeStatus(status) + "</span>"
    // Add the tooltip if the email is scheduled to be sent
    if (status == "Scheduled" || status == "Retrying") {
        var sendDateMessage = "Scheduled to send at " + send_date
        statusColumn = "<span class=\"label " + label + "\" data-toggle=\"tooltip\" data-placement=\"top\" data-html=\"true\" title=\"" + sendDateMessage + "\">" + normalizeStatus(status) + "</span>"
    }
    return statusColumn
}

/* poll - Queries the API and updates the UI with the results
 *
 * Updates:
 * * Timeline Chart
 * * Email (Donut) Chart
 * * Map Bubbles
 * * Datatables
 */
function poll() {
    api.campaignId.results(campaign.id)
        .success(function (c) {
            campaign = c

            var executedEmails = new Set(
                (campaign.timeline || [])
                    .filter(function (ev) { return normalizeStatus(ev.message) === "Executed"; })
                    .map(function (ev) { return ev.email; })
            );

            var trainingCompletedEmails = new Set(
                (campaign.timeline || [])
                    .filter(function (ev) { return normalizeStatus(ev.message) === "Trained"; })
                    .map(function (ev) { return ev.email; })
            );

            /* Update the timeline */
            // (#48) load() 와 동일하게 Campaign Created 는 series 에서 제외.
            // 포함 시 캠페인 생성 시각이 series min 이 되어 X축이 발송 시작 전까지
            // 펼쳐지는 비대칭 결함 (load 직후엔 좁고, Refresh 후엔 넓어짐) 차단.
            var timeline_series_data = []
            $.each(campaign.timeline, function (i, event) {
                if (event.message == "Campaign Created") {
                    return true
                }
                var event_date = moment.utc(event.time).local()
                var meta = getStatusMeta(event.message);
                timeline_series_data.push({
                    email: event.email,
                    message: normalizeStatus(event.message),
                    x: event_date.valueOf(),
                    y: 1,
                    marker: {
                        fillColor: meta.color
                    }
                })
            })

            var timeline_chart = $("#timeline_chart").highcharts()
            timeline_chart.series[0].update({
                data: timeline_series_data
            })

            /* Update the results donut chart */
            var email_series_data = {}
            // Load the initial data
            Object.keys(statusMapping).forEach(function (k) {
                email_series_data[k] = 0
            });

            $.each(campaign.results, function (i, result) {
                var st = normalizeStatus(result.status);
                if (email_series_data[st] === undefined && statusMapping[st]) {
                    email_series_data[st] = 0;
                }
                if (email_series_data[st] !== undefined) {
                    email_series_data[st]++;
                }
                if (result.reported) {
                    email_series_data['Reported']++
                }
                if (st === "Executed" || executedEmails.has(result.email)) {
                    email_series_data['Executed']++
                }
                if (trainingCompletedEmails.has(result.email)) {
                    email_series_data['Trained']++
                }
                var step = progressListing.indexOf(st)
                for (var j = 0; j < step; j++) {
                    email_series_data[progressListing[j]]++
                }
            })

            $.each(email_series_data, function (status, count) {
                if (!(status in statusMapping)) {
                    return true
                }
                var $container = $("#" + statusMapping[status] + "_chart");
                var chart = ($container.highcharts && $container.highcharts()) || null;
                if (!chart) return true; // 컨테이너/차트 없으면 스킵

                var email_data = []
                var total = campaign.results.length || 1; // avoid NaN when 0 results
                // 소수점 1자리 정밀도 유지 (예: 16.5%). tooltip 의 Highcharts.numberFormat 과 일치.
                var pct = Math.round((count / total) * 1000) / 10
                email_data.push({
                    name: status,
                    y: pct,
                    count: count
                })
                email_data.push({
                    name: '',
                    y: 100 - pct
                })
                chart.series[0].update({
                    data: email_data
                })
            })

            /* Update the datatable */
            // (#44) Refresh 시 Details 의 시간 컬럼 (Sent/Opened/Clicked/Submitted/
            // Executed/Trained) 이 미갱신되던 결함 차단. campaign.timeline 을
            // 재인덱싱해서 신규 이벤트의 시각을 행 데이터에 반영한다.
            resultsTable = $("#resultsTable").DataTable()
            var evIdx = indexEventTimesByEmail(campaign.timeline || {})
            resultsTable.rows().every(function (i, tableLoop, rowLoop) {
                var row = this.row(i)
                var rowData = row.data()
                var rid = rowData[0]
                $.each(campaign.results, function (j, result) {
                    if (result.id == rid) {
                        var evRec = evIdx[result.email] || {}
                        rowData[6]  = normalizeStatus(result.status)
                        rowData[7]  = evRec.sent   || ""
                        rowData[8]  = evRec.open   || ""
                        rowData[9]  = evRec.click  || ""
                        rowData[10] = evRec.submit || ""
                        rowData[11] = evRec.exec   || ""
                        rowData[12] = result.reported
                        rowData[13] = evRec.train  || ""
                        rowData[14] = moment(result.send_date).format('MMMM Do YYYY, h:mm:ss a')
                        resultsTable.row(i).data(rowData)
                        if (row.child.isShown()) {
                            $(row.node()).find("#caret").removeClass("fa-caret-right")
                            $(row.node()).find("#caret").addClass("fa-caret-down")
                            row.child(renderTimeline(row.data()))
                        }
                        return false
                    }
                })
            })
            resultsTable.draw(false)
            /* Update the map information */
            updateMap(campaign.results)
            $('[data-toggle="tooltip"]').tooltip()
            $("#refresh_message").hide()
            $("#refresh_btn").show()

            // 수강 현황 탭이 열려 있으면 함께 갱신
            if ($("#tab-video-progress").hasClass("active")) {
                loadVideoProgress();
            }
        })
}

function load() {
    campaign.id = window.location.pathname.split('/').slice(-1)[0]
    var use_map = JSON.parse(localStorage.getItem('sentinel.use_map'))
    api.campaignId.results(campaign.id)
        .success(function (c) {
            campaign = c
            if (campaign) {
                $("title").text(c.name + " - Sentinel")
                $("#loading").hide()
                $("#campaignResults").show()
                // Set the title
                $("#page-title").text("Results for " + c.name)
                if (c.status == "Completed") {
                    $('#complete_button')[0].disabled = true;
                    $('#complete_button').text('Completed!');
                    doPoll = false;
                }
                // Setup viewing the details of a result
                $("#resultsTable").on("click", ".timeline-event-details", function () {
                    // Show the parameters
                    payloadResults = $(this).parent().find(".timeline-event-results")
                    if (payloadResults.is(":visible")) {
                        $(this).find("i").removeClass("fa-caret-down")
                        $(this).find("i").addClass("fa-caret-right")
                        payloadResults.hide()
                    } else {
                        $(this).find("i").removeClass("fa-caret-right")
                        $(this).find("i").addClass("fa-caret-down")
                        payloadResults.show()
                    }
                })
                // Setup the results table
                resultsTable = $("#resultsTable").DataTable({
                    destroy: true,
                    "order": [
                        [2, "asc"]
                    ],
                    columnDefs: [
                        { orderable: false, targets: "no-sort" },
                        { className: "details-control", targets: [1] },
                        { visible: false, targets: [0, 6, 14] },
                        {
                            // Status 컬럼 렌더링
                            render: function(data, type, row) {
                                return createStatusLabel(data, row[14]);
                            },
                            targets: [6]
                        },
                        {
                            className: "text-center",
                            render: function(data, type, row, meta) {
                                if (type !== "display") return data;
                                // 컬럼 인덱스 → 차트 색상 매핑
                                var colColors = {
                                    7:  statuses["Sent"].color,
                                    8:  statuses["Opened"].color,
                                    9:  statuses["Clicked"].color,
                                    10: statuses["Submitted"].color,
                                    11: statuses["Executed"].color,
                                    13: statuses["Trained"].color
                                };
                                var color = colColors[meta.col] || "#1abc9c";
                                return data
                                    ? "<i class='fa fa-check-circle' style='color:" + color + "' title='" + data + "'></i>"
                                    : "<span class='text-muted'>-</span>";
                            },
                            targets: [7, 8, 9, 10, 11, 13]
                        },
                        {
                            className: "text-center",
                            render: function(reported, type, row) {
                                if (type !== "display") return reported;
                                var reportedColor = statuses["Reported"].color;  // #45d6ef
                                if (reported) {
                                    return "<i role='button' class='fa fa-check-circle' " +
                                        "style='color:" + reportedColor + "' " +
                                        "title='클릭하여 신고 취소' " +
                                        "onclick='toggle_report(\"" + row[0] + "\", \"" + campaign.id + "\", true);'></i>";
                                }
                                return "<i role='button' class='fa fa-times-circle text-muted' " +
                                    "title='클릭하여 신고 처리' " +
                                    "onclick='toggle_report(\"" + row[0] + "\", \"" + campaign.id + "\", false);'></i>";
                            },
                            targets: [12]
                        }
                    ]
                });
                resultsTable.clear();

                var executedEmails = new Set(
                    (campaign.timeline || [])
                        .filter(function (ev) { return normalizeStatus(ev.message) === "Executed"; })
                        .map(function (ev) { return ev.email; })
                );

                var trainingCompletedEmails = new Set(
                    (campaign.timeline || [])
                        .filter(function (ev) { return normalizeStatus(ev.message) === "Trained"; })
                        .map(function (ev) { return ev.email; })
                );

                var email_series_data = {}
                var timeline_series_data = []
                Object.keys(statusMapping).forEach(function (k) {
                    email_series_data[k] = 0
                });
                // (#43) indexEventTimesByEmail 은 timeline 전체를 순회하므로 results 루프
                // 밖으로 hoist. 824 results × 1661 events 환경에서 페이지 로드 ~20초 → ~1초.
                var evIdx = indexEventTimesByEmail(campaign.timeline || {});
                $.each(campaign.results, function (i, result) {
                    var st = normalizeStatus(result.status)
                    var evRec = evIdx[result.email] || {};
                    resultsTable.row.add([
                        result.id,
                        "<i id=\"caret\" class=\"fa fa-caret-right\"></i>",
                        escapeHtml(result.name) || "",
                        escapeHtml(result.department) || "",
                        escapeHtml(result.email) || "",
                        escapeHtml(result.position) || "",
                        st,
                        evRec.sent   || "",
                        evRec.open   || "",
                        evRec.click  || "",
                        evRec.submit || "",
                        evRec.exec   || "",
                        result.reported,
                        evRec.train  || "",
                        moment(result.send_date).format('MMMM Do YYYY, h:mm:ss a')
                    ])
                    if (email_series_data[st] === undefined && statusMapping[st]) {
                        email_series_data[st] = 0
                    }
                    if (email_series_data[st] !== undefined) {
                        email_series_data[st]++;
                    }
                    if (result.reported) {
                        email_series_data['Reported']++
                    }
                    if (st === "Executed" || executedEmails.has(result.email)) {
                        email_series_data['Executed']++
                    }
                    if (trainingCompletedEmails.has(result.email)) {
                        email_series_data['Trained']++
                    }

                    // Backfill status values
                    var step = progressListing.indexOf(st)
                    for (var j = 0; j < step; j++) {
                        email_series_data[progressListing[j]]++
                    }
                })
                resultsTable.draw();
                // Setup tooltips
                $('[data-toggle="tooltip"]').tooltip()
                // Setup the individual timelines
                $('#resultsTable tbody').on('click', 'td.details-control', function () {
                    var tr = $(this).closest('tr');
                    var row = resultsTable.row(tr);
                    if (row.child.isShown()) {
                        // This row is already open - close it
                        row.child.hide();
                        tr.removeClass('shown');
                        $(this).find("i").removeClass("fa-caret-down")
                        $(this).find("i").addClass("fa-caret-right")
                    } else {
                        // Open this row
                        $(this).find("i").removeClass("fa-caret-right")
                        $(this).find("i").addClass("fa-caret-down")
                        row.child(renderTimeline(row.data())).show();
                        tr.addClass('shown');
                    }
                });
                // Setup the graphs
                $.each(campaign.timeline, function (i, event) {
                    if (event.message == "Campaign Created") {
                        return true
                    }
                    var event_date = moment.utc(event.time).local()
                    var meta = getStatusMeta(event.message)
                    timeline_series_data.push({
                        email: event.email,
                        message: normalizeStatus(event.message),
                        x: event_date.valueOf(),
                        y: 1,
                        marker: {
                            fillColor: meta.color
                        }
                    })
                })
                renderTimelineChart({
                    data: timeline_series_data
                })
                $.each(email_series_data, function (status, count) {
                    if (!(status in statusMapping)) {
                        return true
                    }
                    // 컨테이너 없으면 렌더 스킵
                    var elemId = statusMapping[status] + '_chart';
                    if (!document.getElementById(elemId)) return true;

                    var email_data = []
                    var total = campaign.results.length || 1; // avoid NaN
                    // 소수점 1자리 정밀도 유지 (예: 16.5%). tooltip 의 Highcharts.numberFormat 과 일치.
                    var pct = Math.round((count / total) * 1000) / 10
                    email_data.push({
                        name: status,
                        y: pct,
                        count: count
                    })
                    email_data.push({
                        name: '',
                        y: 100 - pct
                    })
                    var meta = getStatusMeta(status)
                    renderPieChart({
                        elemId: elemId,
                        title: status,
                        name: status,
                        data: email_data,
                        colors: [meta.color, '#dddddd']
                    })
                })

                if (use_map) {
                    $("#resultsMapContainer").show()
                    map = new Datamap({
                        element: document.getElementById("resultsMap"),
                        responsive: true,
                        fills: {
                            defaultFill: "#ffffff",
                            point: "#283F50"
                        },
                        geographyConfig: {
                            highlightFillColor: "#1abc9c",
                            borderColor: "#283F50"
                        },
                        bubblesConfig: {
                            borderColor: "#283F50"
                        }
                    });
                }
                updateMap(campaign.results)
            }
        })
        .error(function () {
            $("#loading").hide()
            errorFlash(" Campaign not found!")
        })
}

var setRefresh

function refresh() {
    if (!doPoll) {
        return;
    }
    $("#refresh_message").show()
    $("#refresh_btn").hide()
    poll()
    clearTimeout(setRefresh)
    setRefresh = setTimeout(refresh, 60000)
};

function toggle_report(rid, cid, currentlyReported) {
    var action = currentlyReported ? "신고 취소" : "신고 처리";
    var text = currentlyReported
        ? "이 결과의 신고 상태를 취소합니다 (RID: " + rid + ")"
        : "이 결과를 신고 처리합니다 (RID: " + rid + ")";

    Swal.fire({
        title: action + " 하시겠습니까?",
        text: text,
        type: "question",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "확인",
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        showLoaderOnConfirm: true
    }).then(function(result) {
        if (!result.value) return;

        if (currentlyReported) {
            // OFF: API로 reported=false 처리
            api.campaignId.get(cid).success(function(c) {
                // 직접 reported 상태 토글 API 호출
                $.ajax({
                    url: "/api/campaigns/" + cid + "/results/" + rid + "/report",
                    method: "DELETE",
                    beforeSend: function(xhr) {
                        xhr.setRequestHeader("Authorization", "Bearer " + user.api_key);
                    }
                }).done(function() {
                    refresh();
                }).fail(function() {
                    Swal.fire("오류", "신고 취소에 실패했습니다.", "error");
                });
            });
        } else {
            // ON: 기존 방식 (피싱 서버 /report 엔드포인트 호출)
            api.campaignId.get(cid).success(function(c) {
                var report_url = new URL(c.url);
                report_url.pathname = '/report';
                report_url.search = "?rid=" + rid;
                fetch(report_url)
                    .then(function(response) {
                        if (!response.ok) throw new Error("HTTP " + response.status);
                        refresh();
                    })
                    .catch(function(error) {
                        Swal.fire("오류", "신고 처리에 실패했습니다: " + error.message, "error");
                    });
            });
        }
    });
}

$(document).ready(function () {
    Highcharts.setOptions({
        global: {
            useUTC: false
        }
    })
    load();

    // Start the polling loop
    setRefresh = setTimeout(refresh, 60000)

    // 수강 현황 탭 클릭 시 API 호출
    $(document).on('click', '#tab-video-progress-link', function () {
        loadVideoProgress();
    });
})

/* ============================================================
 * ▼▼▼ Events (CSV) 프런트 생성 - 시간 & Open IP 출력 버전
 *    - 각 이벤트는 최초 발생 시간을 기록(여러 번 발생해도 첫 번째)
 *    - 시간 포맷: YYYY-MM-DD HH:mm:ss (로컬)
 *    - Mail Open 시점의 IP(Address)도 함께 출력
 * ============================================================ */

// events_flat CSV: {fields, data} 형태로 빌드하여 Papa.unparse에 전달
function buildEventsFlatCSVData(camp) {
    var fields = [
        "Campaign ID",
        "Campaign Name",
        "R_ID",
        "Name",
        "Department",
        "Email",
        "Position",
        "Sent",
        "Open",
        "IP (Open)",
        "Clicked",
        "Submitted",
        "Executed",
        "Reported",
        "Trained"
    ];

    // 타임라인을 이메일별 '최초 발생 시간' & 'Open IP'로 인덱싱
    var idx = indexEventTimesByEmail((camp && camp.timeline) || []);

    var data = [];
    (camp && camp.results || []).forEach(function (r) {
        var email = r.email || r.Email || "";
        var rec   = idx[email] || {};

        // Sent 시간: 우선 result.send_date, 없으면 timeline의 "Sent"
        var sentTime = r.send_date ? moment(r.send_date).format('YYYY-MM-DD HH:mm:ss')
                                   : (rec.sent || "");

        var row = {
            "Campaign ID": camp.id || "",
            "Campaign Name": camp.name || "",
            "R_ID": getRIDFromResult(r),
            "Name": getFullName(r),
            "Department": r.department || r.Department || "",
            "Email": email,
            "Position": r.position || r.Position || "",
            "Sent": sentTime,
            "Open": rec.open || "",
            "IP (Open)": rec.open_ip || "",
            "Clicked": rec.click || "",
            "Submitted": rec.submit || "",
            "Executed": rec.exec || "",
            "Reported": rec.report || "",
            "Trained": rec.train || ""
        };
        data.push(row);
    });

    return { fields: fields, data: data };
}

// 타임라인에서 이메일별 이벤트 최초발생 시간/열람IP 인덱싱
function indexEventTimesByEmail(timeline) {
    var m = Object.create(null);
    (timeline || []).forEach(function (ev) {
        var email = ev.email;
        if (!email) return;

        if (!m[email]) {
            m[email] = { sent:"", open:"", open_ip:"", click:"", submit:"", exec:"", report:"", train:"" };
        }
        var msg = normalizeStatus(ev.message);
        var ts  = formatEventTime(ev.time); // 로컬 시간 문자열

        switch (msg) {
            case "Sent":
                if (!m[email].sent)   m[email].sent   = ts;
                break;
            case "Opened":
                if (!m[email].open) {
                    m[email].open = ts;
                    // Open 시점 IP 추출 (details 기반, 없으면 비움)
                    m[email].open_ip = extractOpenIP(ev);
                }
                break;
            case "Clicked":
                if (!m[email].click)  m[email].click  = ts;
                break;
            case "Submitted":
                if (!m[email].submit) m[email].submit = ts;
                break;
            case "Executed":
                if (!m[email].exec)   m[email].exec   = ts;
                break;
            case "Reported":
                if (!m[email].report) m[email].report = ts;
                break;
            case "Trained":
                if (!m[email].train)  m[email].train  = ts;
                break;
        }
    });
    return m;
}

// 이벤트 시간 로컬 포맷
function formatEventTime(t) {
    try {
        return moment.utc(t).local().format('YYYY-MM-DD HH:mm:ss');
    } catch (e) {
        return "";
    }
}

// Mail Open 이벤트에서 IP 추출: DB 구조 기준 (details.browser.address 우선)
function extractOpenIP(ev) {
  try {
    var d = typeof ev.details === "string" ? JSON.parse(ev.details) : (ev.details || {});
    var b = d.browser || d.Browser || {};
    var addr = b.address || b.Address || "";

    // 보조: payload 쪽에도 남아있을 수 있으니 백업 경로 탐색
    if (!addr) {
      var p = d.payload || d.Payload || {};
      addr = p.address || p.Address || p.ip || p.IP || "";
    }

    // 배열/다중 헤더(XFF 등) 처리
    if (Array.isArray(addr)) addr = addr[0] || "";
    if (addr && addr.indexOf(",") !== -1) addr = addr.split(",")[0].trim();

    return addr || "";
  } catch (_) {
    return "";
  }
}

// 결과 객체에서 RID / 이름 안전 추출
function getRIDFromResult(r) {
    if (r.rid) return r.rid;
    if (r.RId) return r.RId;
    return "";
}
function getFullName(r) {
    if (r.name) return r.name;
    if (r.Name) return r.Name;
    var fn = r.first_name || r.FirstName || "";
    var ln = r.last_name  || r.LastName  || "";
    return (fn && ln) ? (fn + " " + ln) : (fn || ln);
}

/* ============================================================
 * 수강 현황 탭 — GET /api/campaigns/{id}/video_progress
 * ============================================================ */
function loadVideoProgress() {
    var cid = campaign.id;
    if (!cid) return;

    api.campaignId.videoProgress(cid)
        .success(function (data) {
            data = data || [];

            // 빈 상태: 테이블 숨기고 안내 메시지 노출
            if (data.length === 0) {
                if (videoProgressTable) {
                    videoProgressTable.clear().draw();
                }
                $('#video-progress-table').hide();
                $('#video-progress-empty')
                    .text('수강 기록이 없습니다.')
                    .show();
                return;
            }

            $('#video-progress-empty').hide();
            $('#video-progress-table').show();

            // 첫 호출 시 DataTable 초기화 (결과 목록과 동일 패턴)
            if (!videoProgressTable) {
                videoProgressTable = $('#video-progress-table').DataTable({
                    destroy: true,
                    order: [[0, 'asc']],
                    columnDefs: [
                        { className: 'text-center', targets: [3, 4, 5, 6, 7] },
                        {
                            // 시청 시간 / 전체 시간 (초 → m:ss)
                            render: function (data, type) {
                                if (type !== 'display') return data;
                                return formatSeconds(data);
                            },
                            targets: [3, 4]
                        },
                        {
                            // 진행률 (% → progress bar)
                            render: function (data, type) {
                                if (type !== 'display') return data;
                                return '<div class="progress" style="margin-bottom:0;min-width:80px">' +
                                       '<div class="progress-bar" style="width:' + data + '%">' +
                                       data + '%</div></div>';
                            },
                            targets: [5]
                        },
                        {
                            // 완료 (1/0 → badge)
                            render: function (data, type) {
                                if (type !== 'display') return data;
                                return data
                                    ? '<span class="label label-success"><i class="fa fa-check"></i> 완료</span>'
                                    : '<span class="label label-default">미완료</span>';
                            },
                            targets: [6]
                        },
                        {
                            // 마지막 업데이트 (ISO → YYYY-MM-DD HH:mm)
                            render: function (data, type) {
                                if (type !== 'display') return data;
                                return data
                                    ? moment.utc(data).local().format('YYYY-MM-DD HH:mm')
                                    : '-';
                            },
                            targets: [7]
                        }
                    ]
                });
            }

            // 데이터 갱신 (clear + row.add + draw(false))
            videoProgressTable.clear();
            $.each(data, function (i, row) {
                var pct = +(row.percent * 100).toFixed(1);
                videoProgressTable.row.add([
                    escapeHtml(row.name || ''),
                    escapeHtml(row.department || ''),
                    escapeHtml(row.email || ''),
                    row.seconds_watched || 0,
                    row.duration || 0,
                    pct,
                    row.trained ? 1 : 0,         // ← row.completed → row.trained
                    row.modified_date || ''
                ]);
            });
            videoProgressTable.draw(false);
        })
        .error(function () {
            if (videoProgressTable) {
                videoProgressTable.clear().draw();
            }
            $('#video-progress-table').hide();
            $('#video-progress-empty')
                .text('수강 현황을 불러오는 중 오류가 발생했습니다.')
                .show();
        });
}

// 초(second)를 m:ss 형식으로 변환
function formatSeconds(s) {
    if (!s || s <= 0) return '0:00';
    var m = Math.floor(s / 60);
    var sec = s % 60;
    return m + ':' + String(sec).padStart(2, '0');
}