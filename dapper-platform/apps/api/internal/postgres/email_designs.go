package postgres

import "strings"

// The built-in email designs: the owner's own HTML from the old website, with its %tags% turned into this app's {{placeholders}}, and the
// short emails (dealership invoice, quotation, password reset, invitation) in the card layout the owner chose for the dealership invoice.
// The owner can still change any of them in the dashboard; these are what is sent until they do.

var bookingPlaceholders = []string{"booking_id", "first_name", "name", "phone", "email", "date", "time", "service", "extras", "location", "address", "where",
	"total", "discount", "coupon", "tax", "paid", "due", "payment", "refund", "status", "invoice", "company_name", "company_phone", "company_website", "company_email", "logo_url", "review_link"}

var companyPlaceholders = []string{"company_name", "company_phone", "company_website", "company_email", "logo_url", "year"}

const emailSignoff = `<div style="margin-top: 30px;padding-top: 20px;border-top: 2px solid #eee">
<p style="margin: 0"><strong>Best regards,</strong></p>
<p style="margin: 5px 0 0 0">{{company_name}}</p>
<p style="margin: 5px 0 0 0">Phone: {{company_phone}}</p>
<p style="margin: 5px 0 0 0">Website: {{company_website}}</p>
<p style="margin: 5px 0 0 0">Email: {{company_email}}</p>
</div>`

const emailReminders = `<h3 style="color: #2c3e50;margin-top: 30px">Important Instructions for Your Appointment:</h3>
<ul style="padding-left: 20px">
<li style="margin-bottom: 10px">We offer both mobile detailing services at your location (home, office, or any address you provide) and in-garage services at our facility.</li>
<li style="margin-bottom: 10px">Our team will contact you 2-3 hours before the appointment. Please ensure your vehicle is ready at the agreed location.</li>
<li style="margin-bottom: 10px">Kindly remove all personal belongings from your vehicle prior to our arrival or drop-off.</li>
<li style="margin-bottom: 10px">Thorough Vehicle Inspection will be conducted on-site.</li>
<li style="margin-bottom: 10px">Prices may vary depending on your Vehicle condition.</li>
<li style="margin-bottom: 10px">Our team might suggest Additional Services based on your vehicle's condition, which you can choose to include in your package. (NOTE: This is completely optional)</li>
<li style="margin-bottom: 10px">Extra charges will apply for any additional services requested.</li>
</ul>
<div style="background-color: #fff3cd;border: 1px solid #ffeaa7;padding: 15px;margin: 20px 0;border-radius: 5px">
<p style="margin: 0;font-weight: bold;color: #856404">Important Notice:</p>
<p style="margin: 5px 0 0 0;color: #856404">Extra charges will be applicable for any kind of pet hair and will vary depending on the size and condition of the car.</p>
</div>`

func emailHeader(subtitle string) string {
	sub := ""
	if subtitle != "" {
		sub = `<p style="margin: 5px 0 0 0;color: #666">` + subtitle + `</p>`
	}
	return `<div style="background-color: #f4f4f4;padding: 20px;text-align: center"><img style="max-width: 80px;height: auto;margin-bottom: 10px" src="{{logo_url}}" alt="Dapper Detailing Pros" />
<h1 style="color: #2c3e50;margin: 0">Dapper Detailing Pros</h1>` + sub + `</div>`
}

// A two-column details table for the customer emails.
func detailRows(rows [][2]string, valueStyle map[int]string) string {
	var b strings.Builder
	for i, r := range rows {
		vs := "padding: 5px 0"
		if s, ok := valueStyle[i]; ok {
			vs += ";" + s
		}
		w := ""
		if i == 0 {
			w = ";width: 40%"
		}
		b.WriteString(`<tr><td style="padding: 5px 0;font-weight: bold` + w + `">` + r[0] + `</td><td style="` + vs + `">` + r[1] + `</td></tr>` + "\n")
	}
	return `<table style="width: 100%"><tbody>` + "\n" + b.String() + `</tbody></table>`
}

// The business alert table (red heading bar, striped rows).
func alertTable(heading, colour string, rows [][2]string, highlight map[int]string) string {
	var b strings.Builder
	for i, r := range rows {
		bg := ""
		if i%2 == 0 {
			bg = ` style="background-color: #f8f9fa"`
		}
		vs := "padding: 12px;border: 1px solid #dee2e6"
		if h, ok := highlight[i]; ok {
			bg = ` style="background-color: ` + h + `"`
			vs += ";font-weight: bold"
		}
		w := ""
		if i == 0 {
			w = ";width: 40%"
		}
		b.WriteString(`<tr` + bg + `><td style="padding: 12px;border: 1px solid #dee2e6;font-weight: bold` + w + `">` + r[0] + `</td><td style="` + vs + `">` + r[1] + `</td></tr>` + "\n")
	}
	return `<table style="max-width: 600px;margin: 0 auto;background-color: #ffffff;border-collapse: collapse"><tbody>
<tr><td style="background-color: ` + colour + `;color: #ffffff;padding: 20px;text-align: center;font-size: 18px;font-weight: bold">` + heading + `</td></tr>
<tr><td style="padding: 30px"><table style="width: 100%;border-collapse: collapse;font-size: 14px"><tbody>
` + b.String() + `</tbody></table></td></tr></tbody></table>`
}

func statusBadge(colour, text string) string {
	return `<span style="background-color: ` + colour + `;color: white;padding: 4px 8px;border-radius: 4px;font-size: 12px">` + text + `</span>`
}

// The card layout of the dealership invoice email: logo, a short message, one red button, and the business footer.
func cardEmail(message, button, href string) string {
	btn := ""
	if button != "" {
		btn = `<p style="margin: 24px 0 8px 0"><a href="` + href + `" style="display: inline-block;background-color: #ff0033;color: #ffffff;padding: 14px 28px;font-size: 20px;text-decoration: none">` + button + `</a></p>`
	}
	return `<div style="max-width: 570px;margin: 0 auto;border: 1px solid #ccc;background-color: #ffffff;font-family: Arial, Helvetica, sans-serif;color: #333;text-align: center">
<div style="padding: 24px 24px 40px 24px">
<img src="{{logo_url}}" alt="Dapper Detailing Pros" style="width: 100px;height: auto" />
<div style="margin-top: 50px;font-size: 16px;line-height: 1.6">` + message + `</div>` + btn + `
</div>
<div style="background-color: #f7f7f7;padding: 24px;font-size: 15px;line-height: 2.2">
<strong>{{company_name}}</strong><br />{{company_phone}}<br /><strong>{{company_website}}</strong>
</div>
<div style="background-color: #222;color: #fff;padding: 12px;font-size: 11px">&copy; {{year}} Dapper Detailing Pros, All Rights Reserved</div>
</div>`
}

func init() {
	set := func(key, body string, placeholders []string) {
		d := emailDefaults[key]
		d.body, d.html, d.placeholders = body, true, placeholders
		emailDefaults[key] = d
	}

	// #62 Booking confirmed, to the customer.
	set("booking_confirmation", emailHeader("")+`
<div style="padding: 30px;background-color: #ffffff">
<p>Dear {{first_name}},</p>
<p><strong>Greetings for the day!</strong></p>
<p>We are pleased to confirm that your detailing appointment has been approved by our team. Please review your booking details below:</p>
<table style="width: 100%;border-collapse: collapse;margin: 20px 0;background-color: #f9f9f9"><tbody>
<tr><td style="padding: 10px;border-bottom: 1px solid #ddd;font-weight: bold;width: 40%">Booking ID:</td><td style="padding: 10px;border-bottom: 1px solid #ddd">{{booking_id}}</td></tr>
<tr><td style="padding: 10px;border-bottom: 1px solid #ddd;font-weight: bold">Date:</td><td style="padding: 10px;border-bottom: 1px solid #ddd">{{date}}</td></tr>
<tr><td style="padding: 10px;border-bottom: 1px solid #ddd;font-weight: bold">Time:</td><td style="padding: 10px;border-bottom: 1px solid #ddd">{{time}}</td></tr>
<tr><td style="padding: 10px;border-bottom: 1px solid #ddd;font-weight: bold">Service:</td><td style="padding: 10px;border-bottom: 1px solid #ddd">{{service}}</td></tr>
<tr><td style="padding: 10px;border-bottom: 1px solid #ddd;font-weight: bold">Add-ons:</td><td style="padding: 10px;border-bottom: 1px solid #ddd">{{extras}}</td></tr>
<tr><td style="padding: 10px;border-bottom: 1px solid #ddd;font-weight: bold">Total Amount:</td><td style="padding: 10px;border-bottom: 1px solid #ddd">{{total}}</td></tr>
<tr><td style="padding: 10px;border-bottom: 1px solid #ddd;font-weight: bold">Payment:</td><td style="padding: 10px;border-bottom: 1px solid #ddd">{{payment}}</td></tr>
<tr><td style="padding: 10px;font-weight: bold">Location:</td><td style="padding: 10px">{{where}}</td></tr>
</tbody></table>
`+emailReminders+`
<p>If you have any questions or need to make any changes to your appointment, please do not hesitate to contact us at {{company_phone}} or email us at {{company_email}}.</p>
<p>We look forward to providing you with top-notch detailing service.</p>
<p>Thank you for choosing Dapper Detailing Pros!</p>
`+emailSignoff+`
</div>`, bookingPlaceholders)

	// #67 Booking completed, to the customer.
	set("booking_completed", emailHeader("Service Completed")+`
<div style="padding: 30px;background-color: #ffffff">
<p>Dear {{first_name}},</p>
<p><strong>Thank you for choosing Dapper Detailing Pros!</strong></p>
<p>We are pleased to inform you that your vehicle detailing service has been completed successfully.</p>
<div style="background-color: #f9f9f9;padding: 20px;margin: 20px 0;border-radius: 5px">
<h3 style="margin: 0 0 15px 0;color: #fb1a2b">Service Completion Details:</h3>
`+detailRows([][2]string{{"Booking ID:", "{{booking_id}}"}, {"Service Date:", "{{date}}"}, {"Service Time:", "{{time}}"}, {"Service:", "{{service}}"}, {"Add-ons:", "{{extras}}"},
		{"Total Amount Paid:", "{{paid}}"}, {"Payment Method:", "{{payment}}"}, {"Status:", "COMPLETED"}}, map[int]string{5: "font-weight: bold;color: #fb1a2b", 7: "color: #fb1a2b;font-weight: bold"})+`
</div>
<p>We hope you are satisfied with our service! Your vehicle should now be looking its absolute best.</p>
<div style="background-color: #f8f9fa;border: 1px solid #dee2e6;padding: 20px;margin: 20px 0;border-radius: 5px;text-align: center">
<h3 style="margin: 0 0 10px 0;color: #fb1a2b">Share Your Experience</h3>
<p style="margin: 0 0 15px 0">We would love to hear your feedback! Please consider leaving us a review.</p>
<a style="display: inline-block;background-color: #fb1a2b;color: white;padding: 12px 30px;text-decoration: none;border-radius: 5px;font-weight: bold" href="{{review_link}}">Leave a Review</a>
</div>
<p>If you have any questions about the service provided or need any follow-up assistance, please don't hesitate to contact us at {{company_phone}} or email us at {{company_email}}.</p>
<p>Whether you chose our mobile service or visited our garage, we appreciate your business and look forward to serving you again in the future!</p>
`+emailSignoff+`
</div>`, bookingPlaceholders)

	// #66 Booking rescheduled, to the customer.
	set("booking_rescheduled", emailHeader("Appointment Rescheduled")+`
<div style="padding: 30px;background-color: #ffffff">
<p>Dear {{first_name}}, your detailing appointment has been successfully rescheduled. Please find your updated appointment details below:</p>
<div style="background-color: #f9f9f9;padding: 20px;margin: 20px 0;border-radius: 5px">
<h3 style="margin: 0 0 15px 0;color: #fb1a2b">Updated Appointment Details:</h3>
`+detailRows([][2]string{{"Booking ID:", "{{booking_id}}"}, {"New Date:", "{{date}}"}, {"New Time:", "{{time}}"}, {"Service:", "{{service}}"}, {"Add-ons:", "{{extras}}"},
		{"Total Amount:", "{{total}}"}, {"Location:", "{{where}}"}, {"Status:", "RESCHEDULED"}}, map[int]string{1: "font-weight: bold", 2: "font-weight: bold", 7: "color: #fb1a2b;font-weight: bold"})+`
</div>
`+strings.Replace(emailReminders, "Important Instructions for Your Appointment:", "Important Reminders:", 1)+`
<p>If you need to make further changes, please contact us at {{company_phone}}. We look forward to providing you with top-notch detailing service at your new scheduled time. Thank you for choosing Dapper Detailing Pros!</p>
`+emailSignoff+`
</div>`, bookingPlaceholders)

	// #65 Booking cancelled, to the customer.
	set("booking_cancelled", emailHeader("Appointment Cancellation Notice")+`
<div style="padding: 30px;background-color: #ffffff">
<p>Dear {{first_name}},</p>
<p>We are writing to confirm that your detailing appointment has been cancelled.</p>
<div style="background-color: #f8d7da;border: 1px solid #f5c6cb;padding: 20px;margin: 20px 0;border-radius: 5px">
<h3 style="margin: 0 0 15px 0;color: #721c24">Cancelled Appointment Details:</h3>
`+detailRows([][2]string{{"Booking ID:", "{{booking_id}}"}, {"Date:", "{{date}}"}, {"Time:", "{{time}}"}, {"Service:", "{{service}}"}, {"Add-ons:", "{{extras}}"},
		{"Original Amount:", "{{total}}"}, {"Status:", "CANCELLED"}}, map[int]string{0: "color: #721c24", 6: "color: #721c24;font-weight: bold"})+`
</div>
<p>If a payment was made for this appointment, a refund of <strong>{{refund}}</strong> will be processed within 5-7 business days to your original payment method.</p>
<p>We're sorry to see you go! If you would like to book a new appointment in the future, please visit our website at {{company_website}} or call us at {{company_phone}}. We offer both mobile and in-garage detailing services to suit your needs.</p>
<p>If you did not request this cancellation or have any questions, please contact us immediately at {{company_phone}} or email {{company_email}}.</p>
`+emailSignoff+`
</div>`, bookingPlaceholders)

	// #70 New booking, to the business.
	set("booking_admin", alertTable("New Booking Approved - {{booking_id}}", "#fb1a2b", [][2]string{
		{"Customer Name", "{{name}}"}, {"Customer Phone", "{{phone}}"}, {"Customer Email", "{{email}}"}, {"Date", "{{date}}"}, {"Time", "{{time}}"},
		{"Service Name", "{{service}}"}, {"Service Extras", "{{extras}}"}, {"Discount Amount", "{{discount}}"}, {"Service Location", "{{location}}"}, {"Address", "{{address}}"},
		{"Payment Method", "{{payment}}"}, {"Coupon", "{{coupon}}"}, {"Tax", "{{tax}}"}, {"Appointment TOTAL", "{{total}}"}, {"Appointment Status", statusBadge("#fb1a2b", "{{status}}")},
	}, map[int]string{13: "#ffcccb"}), bookingPlaceholders)

	// #69 Booking cancelled, to the business.
	set("booking_admin_cancelled", alertTable("Booking Cancelled - {{booking_id}}", "#dc3545", [][2]string{
		{"Customer Name", "{{name}}"}, {"Customer Phone", "{{phone}}"}, {"Customer Email", "{{email}}"}, {"Date", "{{date}}"}, {"Time", "{{time}}"},
		{"Service Name", "{{service}}"}, {"Service Extras", "{{extras}}"}, {"Discount Amount", "{{discount}}"}, {"Address", "{{address}}"}, {"Payment Method", "{{payment}}"},
		{"Coupon", "{{coupon}}"}, {"Tax", "{{tax}}"}, {"Refund Amount", `<span style="color: #dc3545">{{refund}}</span>`}, {"Appointment Status", statusBadge("#dc3545", "CANCELLED")},
	}, map[int]string{12: "#f8d7da"}), bookingPlaceholders)

	// Booking moved, to the business: the same table with the new time.
	set("booking_admin_rescheduled", alertTable("Booking Rescheduled - {{booking_id}}", "#fb1a2b", [][2]string{
		{"Customer Name", "{{name}}"}, {"Customer Phone", "{{phone}}"}, {"Customer Email", "{{email}}"}, {"New Date", "{{date}}"}, {"New Time", "{{time}}"},
		{"Service Name", "{{service}}"}, {"Service Extras", "{{extras}}"}, {"Service Location", "{{location}}"}, {"Address", "{{address}}"}, {"Payment Method", "{{payment}}"},
		{"Appointment TOTAL", "{{total}}"}, {"Appointment Status", statusBadge("#fb1a2b", "RESCHEDULED")},
	}, map[int]string{10: "#ffcccb"}), bookingPlaceholders)

	// #71 Dealership invoice.
	set("dealer_invoice_issued", cardEmail(`<p style="margin: 0 0 24px 0">{{name}}</p><p style="margin: 0">To view your invoice for {{total}}, click the link below.</p>`, "View Invoice", "{{link}}"),
		append([]string{"name", "number", "total", "due_date", "link"}, companyPlaceholders...))

	// #73 Quotation, in the same card.
	set("quotation_issued", cardEmail(`<p style="margin: 0 0 24px 0">{{name}}</p><p style="margin: 0">Your quotation {{reference}} for {{total}} is attached to this email.</p><p style="margin: 12px 0 0 0;font-size: 14px;color: #666">{{valid_until}}</p>`, "Call us to go ahead", "tel:+17059776217"),
		append([]string{"name", "reference", "total", "valid_until"}, companyPlaceholders...))

	// #72 Password reset and team invitation, in the same card.
	set("password_reset", cardEmail(`<p style="margin: 0 0 16px 0">Someone asked to reset the password for this email address at Dapper Detailing.</p><p style="margin: 0">Use the button below within one hour to choose a new password.</p><p style="margin: 16px 0 0 0;font-size: 13px;color: #666">If this was not you, ignore this email. Your password has not changed.</p>`, "Choose a new password", "{{link}}"),
		append([]string{"link"}, companyPlaceholders...))
	set("invitation", cardEmail(`<p style="margin: 0 0 16px 0">You have been invited to join {{organization}}.</p><p style="margin: 0">Use the button below to choose your password. After you sign in you will confirm your name and mobile phone, add an authenticator app for two-factor sign-in (required for administrators, optional for employees), and read your company documents.</p><p style="margin: 16px 0 0 0;font-size: 13px;color: #666">This link works once and expires soon. If you were not expecting it, ignore this email.</p>`, "Set up my account", "{{link}}"),
		append([]string{"organization", "link"}, companyPlaceholders...))
}
