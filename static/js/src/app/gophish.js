function errorFlash(message) {
    $("#flashes").empty().append(
        "<div style='text-align:center' class='alert alert-danger'>\
         <i class='fa fa-exclamation-circle'></i> " + message + "</div>"
    );
}
function successFlash(message) {
    $("#flashes").empty().append(
        "<div style='text-align:center' class='alert alert-success'>\
         <i class='fa fa-check-circle'></i> " + message + "</div>"
    );
}
function errorFlashFade(message, fade) {
    errorFlash(message);
    setTimeout(function(){ $("#flashes").empty() }, fade * 1000);
}
function successFlashFade(message, fade) {
    successFlash(message);
    setTimeout(function(){ $("#flashes").empty() }, fade * 1000);
}
function modalError(message) {
    $("#modal\\.flashes").empty().append(
        "<div style='text-align:center' class='alert alert-danger'>\
         <i class='fa fa-exclamation-circle'></i> " + message + "</div>"
    );
}

// --------- 핵심: API URL 생성기 ---------
function apiUrl(endpoint) {
    if (!endpoint) return "/api/";
    if (endpoint.charAt(0) !== "/") endpoint = "/" + endpoint;
    return "/api" + endpoint; // 항상 /api/... 절대경로
}

// --------- 공용 Ajax Wrapper ----------
function query(endpoint, method, data, async) {
    var url = apiUrl(endpoint);
    var m = (method || "GET").toUpperCase();

    var opts = {
        url: url,
        async: !!async,
        method: m,
        beforeSend: function (xhr) {
            xhr.setRequestHeader('Authorization', 'Bearer ' + user.api_key);
        }
    };

    if (m === "POST" || m === "PUT" || m === "PATCH") {
        opts.data = JSON.stringify(data || {});
        opts.dataType = "json";
        opts.contentType = "application/json";
    }
    // GET/DELETE는 body 없이 보냄

    return $.ajax(opts);
}

// 파일 업로드용
function queryForm(endpoint, method, formData, async) {
    return $.ajax({
        url: apiUrl(endpoint),
        async: !!async,
        method: method,
        data: formData,
        processData: false,
        contentType: false,
        beforeSend: function (xhr) {
            xhr.setRequestHeader('Authorization', 'Bearer ' + user.api_key);
        }
    });
}

// HTML escape helpers
function escapeHtml(text) { return $("<div/>").text(text).html(); }
window.escapeHtml = escapeHtml;
function unescapeHtml(html) { return $("<div/>").html(html).text(); }
var capitalize = function (s) { return s.charAt(0).toUpperCase() + s.slice(1); };

// --------- API 엔드포인트 정의 ----------
var api = {
    campaigns: {
        get: function () { return query("campaigns/", "GET", {}, false); },
        post: function (data) { return query("campaigns/", "POST", data, false); },
        summary: function () { return query("campaigns/summary", "GET", {}, false); }
    },
    campaignId: {
        get: function (id) { return query("campaigns/" + id, "GET", {}, true); },
        delete: function (id) { return query("campaigns/" + id, "DELETE", {}, false); },
        results: function (id) { return query("campaigns/" + id + "/results", "GET", {}, true); },
        complete: function (id) { return query("campaigns/" + id + "/complete", "GET", {}, true); },
        summary: function (id) { return query("campaigns/" + id + "/summary", "GET", {}, true); }
    },
    groups: {
        get: function () { return query("groups/", "GET", {}, false); },
        post: function (g) { return query("groups/", "POST", g, false); },
        summary: function () { return query("groups/summary", "GET", {}, true); }
    },
    groupId: {
        get: function (id) { return query("groups/" + id, "GET", {}, false); },
        put: function (g) { return query("groups/" + g.id, "PUT", g, false); },
        delete: function (id) { return query("groups/" + id, "DELETE", {}, false); }
    },
    templates: {
        get: function () { return query("templates/", "GET", {}, false); },
        post: function (t) { return query("templates/", "POST", t, false); }
    },
    templateId: {
        get: function (id) { return query("templates/" + id, "GET", {}, false); },
        put: function (t) { return query("templates/" + t.id, "PUT", t, false); },
        delete: function (id) { return query("templates/" + id, "DELETE", {}, false); }
    },
    pages: {
        get: function () { return query("pages/", "GET", {}, false); },
        post: function (p) { return query("pages/", "POST", p, false); }
    },
    pageId: {
        get: function (id) { return query("pages/" + id, "GET", {}, false); },
        put: function (p) { return query("pages/" + p.id, "PUT", p, false); },
        delete: function (id) { return query("pages/" + id, "DELETE", {}, false); }
    },
    videos: {
        get: function () { return query("videos/", "GET", {}, false); },
        post: function (formData) { return queryForm("videos/", "POST", formData, false); }
    },
    videoId: {
        delete: function (id) { return query("videos/" + id, "DELETE", {}, false); }
    },
    SMTP: {
        get: function () { return query("smtp/", "GET", {}, false); },
        post: function (s) { return query("smtp/", "POST", s, false); }
    },
    SMTPId: {
        get: function (id) { return query("smtp/" + id, "GET", {}, false); },
        put: function (s) { return query("smtp/" + s.id, "PUT", s, false); },
        delete: function (id) { return query("smtp/" + id, "DELETE", {}, false); }
    },
    IMAP: {
        get: function() { return query("imap/", "GET", {}, false); },
        post: function(e) { return query("imap/", "POST", e, false); },
        validate: function(e) { return query("imap/validate", "POST", e, true); }
    },
    users: {
        get: function () { return query("users/", "GET", {}, true); },
        post: function (u) { return query("users/", "POST", u, true); }
    },
    userId: {
        get: function (id) { return query("users/" + id, "GET", {}, true); },
        put: function (u) { return query("users/" + u.id, "PUT", u, true); },
        delete: function (id) { return query("users/" + id, "DELETE", {}, true); }
    },
    webhooks: {
        get: function () { return query("webhooks/", "GET", {}, false); },
        post: function (w) { return query("webhooks/", "POST", w, false); }
    },
    webhookId: {
        get: function (id) { return query("webhooks/" + id, "GET", {}, false); },
        put: function (w) { return query("webhooks/" + w.id, "PUT", w, true); },
        delete: function (id) { return query("webhooks/" + id, "DELETE", {}, false); },
        ping: function (id) { return query("webhooks/" + id + "/validate", "POST", {}, true); }
    },
    import_email: function (req) { return query("import/email", "POST", req, false); },
    clone_site:   function (req) { return query("import/site", "POST", req, false); },
    send_test_email: function (req) { return query("util/send_test_email", "POST", req, true); },
    reset: function () { return query("reset", "POST", {}, true); }
};
window.api = api;

// ---------- UI Helpers ----------
$(document).ready(function () {
    var path = location.pathname;
    $('.nav-sidebar li').each(function () {
        if ($(this).find("a").attr('href') === path) {
            $(this).addClass('active');
        }
    });
    $.fn.dataTable.moment('MMMM Do YYYY, h:mm:ss a');
    $('[data-toggle="tooltip"]').tooltip();
});

