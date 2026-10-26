<?php
// Isolated contract tests with WordPress boundary doubles. Not a live WP test.
define('ABSPATH', __DIR__);
define('ARRAY_A', 'ARRAY_A');
$logged_in = false; $administrator = false; $registered = array();
class WP_Error { public function __construct(public $code, public $message, public $data) {} }
class WP_REST_Server { const READABLE = 'GET'; }
class WP_REST_Response {
    public $headers = array();
    public function __construct(public $data, public $status = 200) {}
    public function header($name, $value) { $this->headers[$name] = $value; }
}
function add_action($event, $callback) {}
function is_user_logged_in() { global $logged_in; return $logged_in; }
function current_user_can($capability) { global $administrator; check($capability === 'manage_options', 'Wrong capability'); return $administrator; }
function register_rest_route($namespace, $path, $args) { global $registered; $registered[$path] = $args; }
function wp_strip_all_tags($value) { return strip_tags(preg_replace('#<(script|style)\b[^>]*>.*?</\1>#is', '', $value)); }
function strip_shortcodes($value) { return preg_replace('/\[[^\]]*\]/', '', $value); }
function home_url($path) { return 'https://example.test/'; }
function wp_parse_url($url) { return parse_url($url); }
function get_bloginfo($key) { return 'Demo'; }
function wp_timezone_string() { return 'America/Toronto'; }
function get_permalink($post) { return 'https://example.test/' . $post->ID; }
function get_post_thumbnail_id($post) { return 10; }
function wp_get_attachment_url($id) { return 'https://example.test/images/' . $id . '.jpg'; }
function get_post_meta($id, $key, $single) { check($key === '_wp_attachment_image_alt', 'Unexpected metadata access'); return 'Public image'; }
function get_post($id) { return (object) array('ID' => $id, 'post_status' => $id === 2 ? 'draft' : 'publish', 'post_password' => $id === 3 ? 'secret' : '', 'post_type' => 'page'); }
class WP_Query {
    public $posts; public $found_posts = 1; public $max_num_pages = 3;
    public function __construct($args) {
        check($args['posts_per_page'] <= 100 && $args['paged'] >= 1, 'Unbounded pagination');
        if ($args['post_type'] === 'attachment') {
            $this->posts = array_map(fn($id) => (object) array('ID' => 10 + $id, 'post_parent' => $id, 'post_title' => 'Image', 'post_mime_type' => 'image/jpeg'), array(1, 2, 3));
        } else {
            check($args['post_status'] === 'publish' && $args['has_password'] === false, 'Private content query');
            $this->posts = array((object) array('ID' => 1, 'post_type' => 'page', 'post_name' => 'services', 'post_title' => '<b>Services</b>', 'post_content' => '<script>secret()</script><p>Hello</p>[private_shortcode]', 'post_modified_gmt' => '2026-10-21'));
        }
    }
}
class FakeDB {
    public $prefix = 'wp_'; public $last_error = ''; public $queries = array();
    public function esc_like($value) { return $value; }
    public function prepare($sql, ...$args) {
        if (count($args) === 1 && is_array($args[0])) $args = $args[0];
        $index = 0;
        $prepared = preg_replace_callback('/%[isd]/', function($match) use (&$index, $args) {
            $value = $args[$index++];
            return $match[0] === '%i' ? '`' . $value . '`' : ($match[0] === '%d' ? (string)(int)$value : "'" . $value . "'");
        }, $sql);
        $this->queries[] = $prepared; return $prepared;
    }
    public function get_var($sql) { if (str_starts_with($sql, 'SHOW TABLES') && str_contains($sql, 'extra_services')) return 'wp_bookingpress_extra_services'; if (str_starts_with($sql, 'SHOW TABLES')) return str_contains($sql, 'categories') ? 'wp_bookingpress_categories' : 'wp_bookingpress_services'; return 1; }
    public function get_col($sql) { return array('bookingpress_service_id', 'bookingpress_service_name', 'bookingpress_service_price', 'bookingpress_category_id', 'bookingpress_category_name', 'api_key', 'customer_email', 'merchant_token', 'bookingpress_extra_services_id', 'bookingpress_extra_service_name', 'bookingpress_extra_service_price', 'bookingpress_extra_service_duration', 'bookingpress_extra_service_duration_unit', 'bookingpress_extra_service_min_quantity', 'bookingpress_extra_service_max_quantity'); }
    public function get_results($sql, $format) {
        preg_match('/^SELECT (.*?) FROM/', $sql, $match);
        $columns = array_map(fn($value) => trim($value, ' `'), explode(',', $match[1]));
        $row = array('bookingpress_service_id' => '1', 'bookingpress_service_name' => 'Demo service', 'bookingpress_service_price' => '49.90', 'bookingpress_category_id' => '1', 'bookingpress_category_name' => 'Demo category', 'api_key' => 'must-not-export', 'customer_email' => 'must-not-export', 'merchant_token' => 'must-not-export', 'bookingpress_extra_services_id' => '7', 'bookingpress_extra_service_name' => 'Demo add-on', 'bookingpress_extra_service_price' => '19.90', 'bookingpress_extra_service_duration' => '15', 'bookingpress_extra_service_duration_unit' => 'm', 'bookingpress_extra_service_min_quantity' => '1', 'bookingpress_extra_service_max_quantity' => '2');
        return array(array_intersect_key($row, array_flip($columns)));
    }
}
function check($condition, $message) { if (!$condition) throw new Exception($message); }
$wpdb = new FakeDB();
require __DIR__ . '/../dapper-data-exporter/dapper-data-exporter.php';
$manifest = Dapper_Data_Exporter::manifest(array());
$plugin_source = file_get_contents(__DIR__ . '/../dapper-data-exporter/dapper-data-exporter.php');
preg_match('/\* Version: ([^\r\n]+)/', $plugin_source, $version_match);
check($manifest->data['exporter_version'] === trim($version_match[1]), 'Manifest version differs from plugin header');
$readme = file_get_contents(__DIR__ . '/../dapper-data-exporter/readme.txt');
check(str_contains($readme, 'Stable tag: ' . $manifest->data['exporter_version']), 'Readme version differs from manifest');
check(Dapper_Data_Exporter::permission()->data['status'] === 401, 'Anonymous access allowed');
$logged_in = true;
check(Dapper_Data_Exporter::permission()->data['status'] === 403, 'Non-admin access allowed');
$administrator = true;
check(Dapper_Data_Exporter::permission() === true, 'Admin rejected');
$cross_origin = new class { public function get_header($name) { return 'https://untrusted.example'; } };
check(Dapper_Data_Exporter::permission($cross_origin)->data['status'] === 403, 'Cross-origin export accepted');
$same_origin = new class { public function get_header($name) { return 'https://example.test'; } };
check(Dapper_Data_Exporter::permission($same_origin) === true, 'Same-origin admin rejected');
$missing_origin = new class { public function get_header($name) { return null; } };
check(Dapper_Data_Exporter::permission($missing_origin) === true, 'Admin GET without Origin header rejected');
Dapper_Data_Exporter::register();
check(count($registered) === 4, 'Unexpected endpoints');
foreach ($registered as $route) { check($route['methods'] === 'GET', 'Write route registered'); check($route['permission_callback'] === array('Dapper_Data_Exporter', 'permission'), 'Unprotected route'); }
$request = array('page' => -5, 'per_page' => 5000);
$content = Dapper_Data_Exporter::content($request);
check($content->data['page'] === 1 && $content->data['per_page'] === 100, 'Pagination not bounded');
check($content->data['items'][0]['text'] === 'Hello', 'Executable content or shortcodes leaked');
check($content->headers['Cache-Control'] === 'private, no-store, max-age=0', 'Cacheable export');
$media = Dapper_Data_Exporter::media($request);
check(count($media->data['items']) === 1 && $media->data['items'][0]['parent_id'] === 1, 'Private attachment exported');
$catalogue = Dapper_Data_Exporter::catalogue($request);
$encoded = json_encode($catalogue->data);
check(!str_contains($encoded, 'must-not-export'), 'Secret column leaked');
check($catalogue->data['resources']['services']['items'][0]['bookingpress_service_price'] === '49.90', 'Decimal price changed');
check($catalogue->data['pricing_approval'] === 'reference_only_pending_client_approval', 'Approval gate lost');
$extra = $catalogue->data['resources']['extras']['items'][0];
check($extra['bookingpress_extra_services_id'] === '7' && $extra['bookingpress_service_id'] === '1', 'Add-on parent identity lost');
check($extra['bookingpress_extra_service_price'] === '19.90', 'Add-on decimal price changed');
check($extra['bookingpress_extra_service_duration_unit'] === 'm' && $extra['bookingpress_extra_service_max_quantity'] === '2', 'Add-on units or quantities lost');
class MissingExtrasDB extends FakeDB { public function get_var($sql) { if (str_starts_with($sql, 'SHOW TABLES') && str_contains($sql, 'extra_services')) return null; return parent::get_var($sql); } }
$wpdb = new MissingExtrasDB();
check(Dapper_Data_Exporter::catalogue($request)->data['resources']['extras']['available'] === false, 'Absent extras table not reported');
class BrokenExtrasDB extends FakeDB { public function get_col($sql) { return str_contains($sql, 'extra_services') ? array('bookingpress_extra_services_id') : parent::get_col($sql); } }
$wpdb = new BrokenExtrasDB();
check(Dapper_Data_Exporter::catalogue($request)->data['status'] === 409, 'Missing add-on parent column accepted');
echo "PASS: authentication/capability, GET-only routes, bounds, public-only content/media, explicit SQL projection, exact decimal references and no-cache headers\n";
