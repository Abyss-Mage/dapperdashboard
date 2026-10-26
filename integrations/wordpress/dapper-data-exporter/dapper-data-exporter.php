<?php
/**
 * Plugin Name: Dapper Data Exporter
 * Description: Administrator-only, read-only exports of published content, images and allowlisted BookingPress catalogue fields.
 * Version: 0.1.3
 * Requires at least: 6.2
 * Requires PHP: 8.0
 * License: GPL-3.0-only
 */
defined('ABSPATH') || exit;

final class Dapper_Data_Exporter {
    const VERSION = '0.1.3';
    const NAMESPACE = 'dapper-export/v1';

    public static function permission($request = null) {
        if (!is_user_logged_in()) {
            return new WP_Error('dapper_auth_required', 'Authenticate with an administrator account.', array('status' => 401));
        }
        if (!current_user_can('manage_options')) {
            return new WP_Error('dapper_forbidden', 'Administrator export permission is required.', array('status' => 403));
        }
        // WP_REST_Request returns null when the header is absent. Authenticated
        // navigations and application-password clients need not send Origin.
        $origin = $request ? (string) $request->get_header('origin') : '';
        $site = wp_parse_url(home_url('/'));
        $expected = $site['scheme'] . '://' . $site['host'] . (isset($site['port']) ? ':' . $site['port'] : '');
        if ($origin !== '' && $origin !== $expected) {
            return new WP_Error('dapper_origin_rejected', 'Cross-origin exports are not allowed.', array('status' => 403));
        }
        return true;
    }

    public static function register() {
        foreach (array('manifest', 'content', 'media', 'catalogue') as $endpoint) {
            register_rest_route(self::NAMESPACE, '/' . $endpoint, array(
                'methods' => WP_REST_Server::READABLE,
                'callback' => array(__CLASS__, $endpoint),
                'permission_callback' => array(__CLASS__, 'permission'),
                'args' => array(
                    'page' => array('default' => 1, 'type' => 'integer', 'minimum' => 1, 'maximum' => 100000),
                    'per_page' => array('default' => 50, 'type' => 'integer', 'minimum' => 1, 'maximum' => 100),
                ),
            ));
        }
    }

    private static function response($data) {
        $response = new WP_REST_Response($data, 200);
        $response->header('Cache-Control', 'private, no-store, max-age=0');
        $response->header('X-Content-Type-Options', 'nosniff');
        return $response;
    }

    private static function page($request) {
        return array(max(1, min(100000, (int) $request['page'])), max(1, min(100, (int) $request['per_page'])));
    }

    private static function text($value) {
        return html_entity_decode(wp_strip_all_tags(strip_shortcodes((string) $value)), ENT_QUOTES | ENT_HTML5, 'UTF-8');
    }

    public static function manifest($request) {
        return self::response(array(
            'schema_version' => 1,
            'exporter_version' => self::VERSION,
            'site_url' => home_url('/'),
            'site_name' => get_bloginfo('name'),
            'timezone' => wp_timezone_string(),
            'generated_at' => gmdate('c'),
            'read_only' => true,
            'resources' => array('content', 'media', 'catalogue'),
            'excludes' => array('users', 'customers', 'bookings', 'payments', 'credentials', 'options', 'private_posts', 'raw_post_meta'),
            'pricing_approval' => 'reference_only_pending_client_approval',
        ));
    }

    public static function content($request) {
        list($page, $limit) = self::page($request);
        $query = new WP_Query(array(
            'post_type' => array('page', 'post'), 'post_status' => 'publish',
            'has_password' => false, 'posts_per_page' => $limit, 'paged' => $page,
            'orderby' => 'ID', 'order' => 'ASC', 'ignore_sticky_posts' => true,
        ));
        $items = array();
        foreach ($query->posts as $post) {
            $items[] = array(
                'source_id' => (int) $post->ID, 'type' => $post->post_type,
                'slug' => $post->post_name, 'url' => get_permalink($post),
                'title' => self::text($post->post_title), 'text' => self::text($post->post_content),
                'modified_gmt' => $post->post_modified_gmt,
                'featured_media_id' => (int) get_post_thumbnail_id($post),
            );
        }
        return self::response(array('page' => $page, 'per_page' => $limit, 'total' => (int) $query->found_posts, 'total_pages' => (int) $query->max_num_pages, 'items' => $items));
    }

    public static function media($request) {
        list($page, $limit) = self::page($request);
        // Page by attachment ID, then include only images attached to a public,
        // non-password-protected parent. Never enumerate unattached ID proofs.
        $query = new WP_Query(array(
            'post_type' => 'attachment', 'post_status' => 'inherit', 'post_mime_type' => 'image',
            'posts_per_page' => $limit, 'paged' => $page, 'orderby' => 'ID', 'order' => 'ASC',
        ));
        $items = array();
        foreach ($query->posts as $post) {
            $parent = get_post($post->post_parent);
            if (!$parent || $parent->post_status !== 'publish' || $parent->post_password !== '' || !in_array($parent->post_type, array('page', 'post'), true)) {
                continue;
            }
            $items[] = array('source_id' => (int) $post->ID, 'parent_id' => (int) $parent->ID,
                'url' => wp_get_attachment_url($post->ID), 'title' => self::text($post->post_title),
                'alt' => self::text(get_post_meta($post->ID, '_wp_attachment_image_alt', true)),
                'mime_type' => $post->post_mime_type);
        }
        return self::response(array('page' => $page, 'per_page' => $limit, 'total_pages' => (int) $query->max_num_pages, 'items' => $items));
    }

    public static function catalogue($request) {
        global $wpdb;
        list($page, $limit) = self::page($request);
        $definitions = array(
            'services' => array('bookingpress_services', array('bookingpress_service_id', 'bookingpress_category_id', 'bookingpress_service_name', 'bookingpress_service_description', 'bookingpress_service_price', 'bookingpress_service_duration_val', 'bookingpress_service_duration_unit')),
            'categories' => array('bookingpress_categories', array('bookingpress_category_id', 'bookingpress_category_name')),
            'extras' => array('bookingpress_extra_services', array('bookingpress_extra_services_id', 'bookingpress_service_id', 'bookingpress_extra_service_name', 'bookingpress_service_description', 'bookingpress_extra_service_price', 'bookingpress_extra_service_duration', 'bookingpress_extra_service_duration_unit', 'bookingpress_extra_service_min_quantity', 'bookingpress_extra_service_max_quantity')),
        );
        $resources = array();
        foreach ($definitions as $resource => $definition) {
            $table = $wpdb->prefix . $definition[0];
            // Identifiers come exclusively from a fixed allowlist, never URL parameters.
            $exists = $wpdb->get_var($wpdb->prepare('SHOW TABLES LIKE %s', $wpdb->esc_like($table)));
            if ($exists !== $table) {
                $resources[$resource] = array('available' => false, 'items' => array());
                continue;
            }
            $columns = $wpdb->get_col($wpdb->prepare('SHOW COLUMNS FROM %i', $table));
            $allowed = array_values(array_intersect($definition[1], $columns));
            if (!in_array($definition[1][0], $allowed, true)) {
                return new WP_Error('dapper_unsupported_schema', 'BookingPress catalogue schema needs review.', array('status' => 409));
            }
            if ($resource === 'extras' && !in_array('bookingpress_service_id', $allowed, true)) {
                return new WP_Error('dapper_unsupported_schema', 'BookingPress add-on parent mapping needs review.', array('status' => 409));
            }
            $placeholders = implode(', ', array_fill(0, count($allowed), '%i'));
            $arguments = array_merge($allowed, array($table, $definition[1][0], $limit, ($page - 1) * $limit));
            $sql = $wpdb->prepare('SELECT ' . $placeholders . ' FROM %i ORDER BY %i ASC LIMIT %d OFFSET %d', $arguments);
            $rows = $wpdb->get_results($sql, ARRAY_A);
            if ($wpdb->last_error) {
                return new WP_Error('dapper_catalogue_unavailable', 'Could not read catalogue. Review the server logs.', array('status' => 503));
            }
            foreach ($rows as &$row) {
                foreach ($row as &$value) {
                    // Preserve legacy decimal prices as strings; never float-convert money.
                    $value = self::text($value);
                }
                unset($value);
            }
            unset($row);
            $total = (int) $wpdb->get_var($wpdb->prepare('SELECT COUNT(*) FROM %i', $table));
            $resources[$resource] = array('available' => true, 'total' => $total, 'total_pages' => (int) ceil($total / $limit), 'items' => $rows);
        }
        return self::response(array('page' => $page, 'per_page' => $limit, 'pricing_approval' => 'reference_only_pending_client_approval', 'resources' => $resources));
    }
}
add_action('rest_api_init', array('Dapper_Data_Exporter', 'register'));
