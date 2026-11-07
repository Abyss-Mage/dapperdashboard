/* The three legal pages, carried over word for word from the published Dapper site (dapperdetailing.ca). The wording is the legacy text and still needs the owner's approval before launch. */

export type LegalBlock = { p: string } | { ul: string[] };
export type LegalPage = { title: string; lede: string; effective: string; intro: string[]; sections: { h: string; blocks: LegalBlock[] }[] };

export const legal: Record<'privacy-policy' | 'terms-conditions' | 'refund-and-cancellation', LegalPage> = {
  "privacy-policy": {
    "title": "Privacy policy",
    "lede": "How Dapper Detailing Pros collects, uses and protects your personal information.",
    "effective": "May 30, 2024",
    "intro": [
      "Dapper Detailing Pros LTD. (“Dapper Detailing Pros”, “we”, “our”, or “us”) respects your privacy and is committed to protecting your personal information in accordance with the Personal Information Protection and Electronic Documents Act (PIPEDA) and applicable Canadian privacy laws.",
      "This Privacy Policy explains how we collect, use, disclose, store, and protect your personal information when you visit our website, create an account, book our services, or otherwise interact with us."
    ],
    "sections": [
      {
        "h": "1. Company Information",
        "blocks": [
          {
            "p": "Dapper Detailing Pros LTD."
          },
          {
            "p": "660 The Kingsway\nPeterborough, ON K9J 7C8\nCanada"
          },
          {
            "p": "Email:\ninfo@dapperdetailing.ca\ndetailingdapper@gmail.com"
          },
          {
            "p": "Phone:\n+1 (705) 977-6217"
          },
          {
            "p": "Website:\nhttps://dapperdetailing.ca/"
          }
        ]
      },
      {
        "h": "2. Information We Collect",
        "blocks": [
          {
            "p": "We may collect the following personal information:"
          },
          {
            "p": "Account Information"
          },
          {
            "ul": [
              "Username",
              "Email address",
              "Mobile phone number (where provided for SMS communications)",
              "Account login information",
              "Booking history"
            ]
          },
          {
            "p": "Booking Information"
          },
          {
            "p": "When you request vehicle detailing services, we may collect:"
          },
          {
            "ul": [
              "Full name",
              "Email address",
              "Mobile phone number (where provided for SMS communications)",
              "Service location or customer address",
              "Appointment details"
            ]
          },
          {
            "p": "Automatically Collected Information"
          },
          {
            "p": "When you visit our website, we may automatically collect:"
          },
          {
            "ul": [
              "IP address",
              "Browser type",
              "Device information",
              "Operating system",
              "Referring website",
              "Pages viewed",
              "Date and time of visits",
              "Website usage data"
            ]
          }
        ]
      },
      {
        "h": "3. How We Use Your Information",
        "blocks": [
          {
            "p": "We use your information to:"
          },
          {
            "ul": [
              "Create and manage user accounts",
              "Process service bookings",
              "Provide customer suppor",
              "Communicate regarding appointments and services",
              "Send booking confirmations, appointment reminders, service updates, and SMS communications where you have provided your consent.",
              "Improve website performance and user experience",
              "Analyze website traffic and visitor behavior",
              "Send promotional emails and newsletters (with consent)",
              "Comply with legal obligations"
            ]
          }
        ]
      },
      {
        "h": "4. Mobile Messaging Privacy",
        "blocks": [
          {
            "p": "If you provide your mobile phone number and consent to receive SMS messages from Dapper Detailing Pros LTD., we will use your mobile number only for the purposes described in our Terms & Conditions and for communications you have agreed to receive, such as appointment reminders, booking updates, customer support, and promotional messages where applicable."
          },
          {
            "p": "No mobile information will be shared or sold to third parties or affiliates for marketing or promotional purposes."
          },
          {
            "p": "All the above categories exclude text messaging originator opt-in data and consent; this information will not be shared with any third parties."
          },
          {
            "p": "You may opt out of receiving SMS messages at any time by replying STOP to any message. For assistance, reply HELP or contact us at info@dapperdetailing.ca."
          }
        ]
      },
      {
        "h": "5. Email Marketing",
        "blocks": [
          {
            "p": "We may send promotional emails, newsletters, offers, and updates through Mailchimp."
          },
          {
            "p": "You may unsubscribe from marketing communications at any time by:"
          },
          {
            "ul": [
              "Clicking the unsubscribe link included in emails; or",
              "Contacting us directly."
            ]
          },
          {
            "p": "Transactional communications related to bookings or account management may still be sent when necessary."
          }
        ]
      },
      {
        "h": "6. Cookies and Tracking Technologies",
        "blocks": [
          {
            "p": "Our website uses cookies and similar technologies to improve functionality, analyze website performance, and enhance user experience."
          },
          {
            "p": "Cookies may be used for:"
          },
          {
            "ul": [
              "Website functionality",
              "User authentication",
              "Analytics and performance monitoring",
              "Marketing and advertising measurement"
            ]
          },
          {
            "p": "You may disable cookies through your browser settings; however, some features of the website may not function properly."
          }
        ]
      },
      {
        "h": "7. Analytics and Advertising Technologies",
        "blocks": [
          {
            "p": "We use third-party analytics and advertising tools, including:"
          },
          {
            "ul": [
              "Google Analytics",
              "Google Tag Manager",
              "Meta (Facebook) Pixel"
            ]
          },
          {
            "p": "These services help us understand visitor behavior, improve our services, and measure advertising effectiveness."
          },
          {
            "p": "These providers may collect information about your interactions with our website in accordance with their own privacy policies."
          }
        ]
      },
      {
        "h": "8. Payment Processing",
        "blocks": [
          {
            "p": "Payments made through our website are processed securely by Square."
          },
          {
            "p": "We do not collect, store, or have access to your complete payment card information."
          },
          {
            "p": "All payment information is processed directly by Square in accordance with its own privacy and security standards."
          }
        ]
      },
      {
        "h": "9. Third-Party Services",
        "blocks": [
          {
            "p": "Our website may use or integrate third-party services including:"
          },
          {
            "ul": [
              "Square",
              "Mailchimp",
              "Google Maps",
              "Google Analytics",
              "Google Tag Manager",
              "Meta Pixel"
            ]
          },
          {
            "p": "These providers may collect and process information in accordance with their own privacy policies."
          }
        ]
      },
      {
        "h": "10. International Data Processing",
        "blocks": [
          {
            "p": "Some third-party service providers used by our business may store or process information outside Canada."
          },
          {
            "p": "By using our website and services, you acknowledge that information may be transferred to jurisdictions outside Canada where privacy laws may differ from those in Canada."
          },
          {
            "p": "We take reasonable steps to ensure that personal information remains protected through contractual and organizational safeguards."
          }
        ]
      },
      {
        "h": "11. Data Retention",
        "blocks": [
          {
            "p": "We retain personal information only for as long as necessary to:"
          },
          {
            "ul": [
              "Provide our services",
              "Maintain customer accounts",
              "Fulfill legal obligations",
              "Resolve disputes",
              "Enforce agreements"
            ]
          },
          {
            "p": "Account-related information may be retained while your account remains active or until a valid deletion request is received."
          }
        ]
      },
      {
        "h": "12. Your Privacy Rights",
        "blocks": [
          {
            "p": "Subject to applicable law, you may request:"
          },
          {
            "ul": [
              "Provide our services",
              "Correction of inaccurate information",
              "Deletion of personal information",
              "Information regarding how your data is used"
            ]
          },
          {
            "p": "To make a request, please contact us using the details provided below."
          }
        ]
      },
      {
        "h": "13. Security",
        "blocks": [
          {
            "p": "We implement reasonable administrative, technical, and physical safeguards to protect personal information against unauthorized access, disclosure, alteration, misuse, or loss."
          },
          {
            "p": "While we strive to protect your information, no internet transmission or electronic storage system can be guaranteed to be completely secure."
          }
        ]
      },
      {
        "h": "14. Children’s Privacy",
        "blocks": [
          {
            "p": "Our services are available to individuals of all ages. However, we do not knowingly collect personal information from children in violation of applicable privacy laws."
          },
          {
            "p": "If a parent or guardian believes that personal information has been submitted by a child improperly, they may contact us for assistance."
          }
        ]
      },
      {
        "h": "15. Changes to This Privacy Policy",
        "blocks": [
          {
            "p": "We may update this Privacy Policy from time to time."
          },
          {
            "p": "Any changes will be posted on this page together with an updated Effective Date."
          },
          {
            "p": "Continued use of our website after changes are posted constitutes acceptance of the revised Privacy Policy."
          }
        ]
      },
      {
        "h": "16. Contact Us",
        "blocks": [
          {
            "p": "If you have any questions regarding this Privacy Policy or wish to exercise your privacy rights, please contact us:"
          },
          {
            "p": "Dapper Detailing Pros LTD."
          },
          {
            "p": "660 The Kingsway\nPeterborough, ON K9J 7C8\nCanada"
          },
          {
            "p": "Email:\ninfo@dapperdetailing.ca\ndetailingdapper@gmail.com"
          },
          {
            "p": "Phone:\n+1 (705) 977-6217"
          }
        ]
      }
    ]
  },
  "terms-conditions": {
    "title": "Terms & conditions",
    "lede": "The terms that apply when you use the Dapper website and book our services.",
    "effective": "May 30, 2024",
    "intro": [
      "These Terms & Conditions (“Terms”) govern your use of the Dapper Detailing Pros LTD. website and services. By creating an account, booking a service, making a payment, or otherwise using our website or services, you agree to be bound by these Terms."
    ],
    "sections": [
      {
        "h": "1. Company Information",
        "blocks": [
          {
            "p": "Dapper Detailing Pros LTD."
          },
          {
            "p": "660 The Kingsway\nPeterborough, ON K9J 7C8\nCanada"
          },
          {
            "p": "Email:\ninfo@dapperdetailing.ca\ndetailingdapper@gmail.com"
          },
          {
            "p": "Phone:\n+1 (705) 977-6217"
          },
          {
            "p": "Website:\nhttps://dapperdetailing.ca/"
          }
        ]
      },
      {
        "h": "2. Services",
        "blocks": [
          {
            "p": "Dapper Detailing Pros provides professional vehicle detailing services, including but not limited to interior detailing, exterior detailing, ceramic coatings, paint protection, and related automotive care services."
          },
          {
            "p": "All services are subject to availability and may vary depending on vehicle condition, size, accessibility, and environmental factors."
          }
        ]
      },
      {
        "h": "3. User Accounts",
        "blocks": [
          {
            "p": "Users may create an account on our website to manage bookings and access certain features."
          },
          {
            "p": "You are responsible for:"
          },
          {
            "ul": [
              "Maintaining the confidentiality of your account credentials.",
              "Providing accurate and current information.",
              "All activities occurring under your account."
            ]
          },
          {
            "p": "We reserve the right to suspend or terminate accounts that violate these Terms or are used fraudulently."
          }
        ]
      },
      {
        "h": "4. Booking Services",
        "blocks": [
          {
            "p": "Bookings may be made through our website or by contacting our team directly."
          },
          {
            "p": "By submitting a booking request, you agree that:"
          },
          {
            "ul": [
              "All information provided is accurate.",
              "You have authority to schedule service for the vehicle.",
              "The vehicle will be available at the scheduled time and location."
            ]
          },
          {
            "p": "Booking confirmations, appointment reminders, and service updates may be provided via email, SMS, or both, where you have consented to receive SMS communications."
          }
        ]
      },
      {
        "h": "5. Pricing and Estimates",
        "blocks": [
          {
            "p": "Prices displayed on our website are estimates and may vary based on:"
          },
          {
            "ul": [
              "Vehicle size",
              "Vehicle condition",
              "Excessive dirt, stains, pet hair, odors, or contamination",
              "Additional services requested",
              "Accessibility requirements"
            ]
          },
          {
            "p": "Any additional charges will be discussed with the customer before work begins whenever reasonably possible."
          }
        ]
      },
      {
        "h": "6. Payment Terms",
        "blocks": [
          {
            "p": "Payments are securely processed through Square."
          },
          {
            "p": "We do not store or process payment card information directly."
          },
          {
            "p": "Payment may be required:"
          },
          {
            "ul": [
              "At the time of booking;",
              "Before service begins; or",
              "Upon completion of service;"
            ]
          },
          {
            "p": "depending on the service selected and booking arrangement."
          },
          {
            "p": "Failure to complete payment may result in cancellation of services or future booking restrictions."
          }
        ]
      },
      {
        "h": "7. SMS Messaging Terms",
        "blocks": [
          {
            "p": "Purpose of SMS Messages"
          },
          {
            "p": "By providing your mobile phone number and opting in to receive text messages from Dapper Detailing Pros LTD., you consent to receive SMS messages related to:"
          },
          {
            "ul": [
              "Appointment confirmations",
              "Appointment reminders",
              "Booking updates",
              "Service notifications",
              "Promotional offers and marketing campaigns (where you have provided consent)",
              "Customer support communications"
            ]
          },
          {
            "p": "Message Frequency\nMessage frequency may vary depending on your interactions with our services, appointments, and promotional preferences."
          },
          {
            "p": "Message & Data Rates\nMessage and data rates may apply according to your mobile carrier’s plan."
          },
          {
            "p": "Opt-Out\nYou may opt out of receiving SMS messages at any time by replying STOP to any text message you receive from us."
          },
          {
            "p": "After opting out, you will no longer receive SMS messages unless you opt in again."
          },
          {
            "p": "Help\nFor assistance, reply HELP to any SMS message or contact us at:"
          },
          {
            "p": "Email: info@dapperdetailing.ca"
          },
          {
            "p": "Phone: +1 (705) 977-6217"
          },
          {
            "p": "Consent\nYour consent to receive SMS messages is not a condition of purchasing any goods or services."
          },
          {
            "p": "Privacy\nYour use of our SMS services is also governed by our Privacy Policy, available at:"
          },
          {
            "p": "https://dapperdetailing.ca/privacy-policy/"
          }
        ]
      },
      {
        "h": "8. Cancellations and Rescheduling",
        "blocks": [
          {
            "p": "Customers may request cancellation or rescheduling by contacting our team."
          },
          {
            "p": "Rescheduling requests are subject to scheduling availability."
          },
          {
            "p": "We reserve the right to reschedule appointments due to:"
          },
          {
            "ul": [
              "Severe weather conditions",
              "Equipment issues",
              "Safety concerns",
              "Circumstances beyond our reasonable control"
            ]
          },
          {
            "p": "Repeated cancellations or no-shows may result in booking restrictions."
          }
        ]
      },
      {
        "h": "9. Vehicle Condition and Customer Responsibilities",
        "blocks": [
          {
            "p": "Customers are responsible for:"
          },
          {
            "ul": [
              "Providing safe access to the vehicle.",
              "Removing valuable personal belongings before service.",
              "Informing us of any existing damage or concerns."
            ]
          },
          {
            "p": "We are not responsible for:"
          },
          {
            "ul": [
              "Personal items left inside the vehicle.",
              "Pre-existing vehicle damage.",
              "Damage resulting from undisclosed defects, aftermarket modifications, or deteriorated materials."
            ]
          }
        ]
      },
      {
        "h": "10. Service Completion and Satisfaction",
        "blocks": [
          {
            "p": "We strive to provide high-quality detailing services."
          },
          {
            "p": "If you are dissatisfied with a service, you must notify us within 24 hours of service completion."
          },
          {
            "p": "We may, at our discretion:"
          },
          {
            "ul": [
              "Re-inspect the vehicle;",
              "Perform corrective work;",
              "Offer a reasonable resolution."
            ]
          },
          {
            "p": "Refunds are not guaranteed and will be evaluated on a case-by-case basis."
          }
        ]
      },
      {
        "h": "11. Website Use",
        "blocks": [
          {
            "p": "You agree not to:"
          },
          {
            "ul": [
              "Use the website unlawfully.",
              "Attempt unauthorized access to systems or accounts.",
              "Distribute malware or harmful software.",
              "Interfere with website operation."
            ]
          },
          {
            "p": "We reserve the right to restrict access to users who violate these Terms."
          }
        ]
      },
      {
        "h": "12. Intellectual Property",
        "blocks": [
          {
            "p": "All website content, including text, graphics, logos, images, designs, and branding, is the property of Dapper Detailing Pros LTD. or its licensors."
          },
          {
            "p": "You may not copy, reproduce, distribute, modify, or commercially exploit any content without prior written permission."
          }
        ]
      },
      {
        "h": "13. Limitation of Liability",
        "blocks": [
          {
            "p": "To the maximum extent permitted by law, Dapper Detailing Pros LTD. shall not be liable for:"
          },
          {
            "ul": [
              "Indirect damages;",
              "Consequential damages;",
              "Loss of profits;",
              "Loss of business opportunities;",
              "Loss of data;",
              "Incidental damages arising from use of our services or website."
            ]
          },
          {
            "p": "Our total liability shall not exceed the amount paid for the specific service giving rise to the claim."
          }
        ]
      },
      {
        "h": "14. Third-Party Services",
        "blocks": [
          {
            "p": "Our website may integrate third-party services including:"
          },
          {
            "ul": [
              "Square",
              "Mailchimp",
              "Google Maps",
              "Google Analytics",
              "Meta Pixel"
            ]
          },
          {
            "p": "We are not responsible for the policies, practices, or content of third-party providers."
          }
        ]
      },
      {
        "h": "15. Privacy",
        "blocks": [
          {
            "p": "Your use of our website and services is also governed by our Privacy Policy."
          },
          {
            "p": "By using our website, you consent to the collection and use of information as described in our Privacy Policy."
          }
        ]
      },
      {
        "h": "16. Force Majeure",
        "blocks": [
          {
            "p": "We shall not be liable for delays or failures resulting from events beyond our reasonable control, including:"
          },
          {
            "ul": [
              "Severe weather",
              "Natural disasters",
              "Government actions",
              "Utility outages",
              "Internet disruptions",
              "Labor disputes"
            ]
          }
        ]
      },
      {
        "h": "17. Changes to These Terms",
        "blocks": [
          {
            "p": "We reserve the right to update or modify these Terms at any time."
          },
          {
            "p": "Updated Terms will be posted on our website with a revised Effective Date."
          },
          {
            "p": "Continued use of our website or services after changes are posted constitutes acceptance of the updated Terms."
          }
        ]
      },
      {
        "h": "18. Governing Law",
        "blocks": [
          {
            "p": "These Terms shall be governed by and interpreted in accordance with the laws of the Province of Ontario and the laws of Canada applicable therein."
          },
          {
            "p": "Any disputes arising from these Terms shall be subject to the exclusive jurisdiction of the courts located in Ontario, Canada."
          }
        ]
      },
      {
        "h": "19. Contact Information",
        "blocks": [
          {
            "p": "For questions regarding these Terms, please contact:"
          },
          {
            "p": "Dapper Detailing Pros LTD."
          },
          {
            "p": "660 The Kingsway\nPeterborough, ON K9J 7C8\nCanada"
          },
          {
            "p": "Email:\ndetailingdapper@gmail.com"
          },
          {
            "p": "Customer Support Email:\ninfo@dapperdetailing.ca\nThis satisfies the requirement for a customer support contact email."
          },
          {
            "p": "Phone:\n+1 (705) 977-6217"
          }
        ]
      }
    ]
  },
  "refund-and-cancellation": {
    "title": "Refund & cancellation policy",
    "lede": "How cancellations, rescheduling, refunds and service concerns are handled.",
    "effective": "May 30, 2024",
    "intro": [
      "Dapper Detailing Pros LTD. (“Dapper Detailing Pros”, “Company”, “we”, “our”, or “us”) is committed to providing reliable, high-quality vehicle detailing services. This Refund & Cancellation Policy outlines the terms governing cancellations, rescheduling, refunds, and service-related concerns.",
      "By booking a service with Dapper Detailing Pros, you acknowledge and agree to this Policy."
    ],
    "sections": [
      {
        "h": "1. General Policy",
        "blocks": [
          {
            "p": "When a booking is confirmed, we reserve dedicated time, personnel, equipment, and resources specifically for your appointment."
          },
          {
            "p": "As a result, cancellations, rescheduling requests, and refund eligibility are governed by the terms outlined below."
          }
        ]
      },
      {
        "h": "2. Rescheduling Appointments",
        "blocks": [
          {
            "p": "Customers may request to reschedule a booking by contacting us via phone, email, or other approved communication methods."
          },
          {
            "p": "2.1 Rescheduling Requests"
          },
          {
            "ul": [
              "Rescheduling requests made before the scheduled appointment time may be accommodated without penalty.",
              "All rescheduling requests remain subject to scheduling availability.",
              "We will make reasonable efforts to accommodate preferred dates and times."
            ]
          },
          {
            "p": "2.2 Company Rescheduling"
          },
          {
            "p": "We reserve the right to reschedule appointments due to:"
          },
          {
            "ul": [
              "Severe weather conditions",
              "Equipment malfunction",
              "Staff availability issues",
              "Safety concerns",
              "Circumstances beyond our reasonable control"
            ]
          },
          {
            "p": "Customers will be notified as soon as reasonably possible."
          }
        ]
      },
      {
        "h": "3. Customer-Initiated Cancellations",
        "blocks": [
          {
            "p": "3.1 Cancellation Fee"
          },
          {
            "p": "A cancellation fee of CAD $100 may be applied to confirmed bookings cancelled by the customer."
          },
          {
            "p": "3.2 Cancellation Requests"
          },
          {
            "p": "Cancellation requests must be submitted through our approved communication channels."
          },
          {
            "p": "3.3 No Refund for Customer Cancellations"
          },
          {
            "p": "Any deposits, booking fees, or advance payments may be non-refundable following cancellation, subject to applicable law."
          },
          {
            "p": "3.4 Repeated Cancellations"
          },
          {
            "p": "We reserve the right to refuse future bookings from customers who repeatedly cancel scheduled appointments."
          }
        ]
      },
      {
        "h": "4. No-Show Policy",
        "blocks": [
          {
            "p": "A customer will be considered a no-show if:"
          },
          {
            "ul": [
              "They are not present at the scheduled appointment time; or",
              "They fail to provide reasonable access to the vehicle or service location; or",
              "Required arrangements have not been made for service completion."
            ]
          },
          {
            "p": "No-shows may be treated as customer cancellations and may incur a CAD $100 cancellation fee."
          }
        ]
      },
      {
        "h": "5. Company-Initiated Cancellations",
        "blocks": [
          {
            "p": "If Dapper Detailing Pros is unable to perform the booked service due to operational reasons, weather conditions, equipment failure, safety concerns, or other unforeseen circumstances:"
          },
          {
            "p": "5.1 Customer Options"
          },
          {
            "p": "The customer may choose to:"
          },
          {
            "ul": [
              "Reschedule the appointment at no additional charge; or",
              "Receive a full refund of any prepaid amount related to the affected booking."
            ]
          }
        ]
      },
      {
        "h": "6. Refund Policy",
        "blocks": [
          {
            "p": "6.1 General Rule"
          },
          {
            "p": "All completed detailing services are considered final."
          },
          {
            "p": "Due to the nature of detailing services, refunds are generally not provided once services have been completed."
          },
          {
            "p": "6.2 Eligible Refund Circumstances"
          },
          {
            "p": "Refunds may be considered only if:"
          },
          {
            "ul": [
              "Payment was collected but the booked service was not performed;",
              "A duplicate payment was processed in error;",
              "The Company cancels a prepaid booking and the customer declines rescheduling;",
              "A refund is otherwise required by applicable law."
            ]
          },
          {
            "p": "6.3 Refund Processing"
          },
          {
            "p": "Approved refunds will be processed using the original payment method whenever possible."
          },
          {
            "p": "Processing times may vary depending on financial institutions and payment providers."
          }
        ]
      },
      {
        "h": "7. Service Satisfaction",
        "blocks": [
          {
            "p": "Customer satisfaction is important to us."
          },
          {
            "p": "If you believe a service was not completed satisfactorily, you must notify us within 24 hours of service completion."
          },
          {
            "p": "7.1 Resolution Process"
          },
          {
            "p": "At our discretion, we may:"
          },
          {
            "ul": [
              "Inspect the vehicle;",
              "Review photographs or documentation;",
              "Re-perform specific aspects of the service;",
              "Offer another reasonable resolution."
            ]
          },
          {
            "p": "7.2 No Automatic Refund"
          },
          {
            "p": "Service-related concerns do not automatically qualify for a refund."
          },
          {
            "p": "The Company reserves the right to attempt corrective action before considering any other remedy."
          }
        ]
      },
      {
        "h": "8. Vehicle Condition Disclaimer",
        "blocks": [
          {
            "p": "Detailing services improve appearance but cannot guarantee the removal of:"
          },
          {
            "ul": [
              "Permanent stains",
              "Paint defects",
              "Scratches",
              "Oxidation",
              "Rust",
              "Wear and tear",
              "Existing damage"
            ]
          },
          {
            "p": "No refund shall be provided where results are limited due to the condition of the vehicle."
          }
        ]
      },
      {
        "h": "9. Payment Disputes and Chargebacks",
        "blocks": [
          {
            "p": "Customers agree to contact Dapper Detailing Pros directly regarding any billing concerns before initiating a chargeback or payment dispute."
          },
          {
            "p": "We reserve the right to provide booking records, communications, invoices, photographs, and service documentation to payment processors and financial institutions in response to disputes."
          }
        ]
      },
      {
        "h": "10. Limitation of Liability",
        "blocks": [
          {
            "p": "To the fullest extent permitted by law, Dapper Detailing Pros LTD.’s total liability relating to any booking, service, refund request, or dispute shall not exceed the amount paid by the customer for the specific service giving rise to the claim."
          },
          {
            "p": "Under no circumstances shall the Company be liable for indirect, incidental, special, or consequential damages."
          }
        ]
      },
      {
        "h": "11. Changes to This Policy",
        "blocks": [
          {
            "p": "We reserve the right to update or modify this Refund & Cancellation Policy at any time."
          },
          {
            "p": "Any revisions will be posted on our website together with an updated Effective Date."
          },
          {
            "p": "Continued use of our services after changes are posted constitutes acceptance of the revised Policy."
          }
        ]
      },
      {
        "h": "12. Contact Us",
        "blocks": [
          {
            "p": "For questions regarding cancellations, refunds, or rescheduling, please contact:"
          },
          {
            "p": "Dapper Detailing Pros LTD."
          },
          {
            "p": "660 The Kingsway\nPeterborough, ON K9J 7C8\nCanada"
          },
          {
            "p": "Email:\ninfo@dapperdetailing.ca\ndetailingdapper@gmail.com"
          },
          {
            "p": "Phone:\n+1 (705) 977-6217"
          },
          {
            "p": "Website:\nhttps://dapperdetailing.ca/"
          }
        ]
      }
    ]
  }
};
