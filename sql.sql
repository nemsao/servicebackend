-- Create database
--CREATE DATABASE ticket_selling_app;
--\c ticket_selling_app;

-- Enable UUID generation
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ========================
-- User Management Section
-- ========================

CREATE TABLE public.users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username VARCHAR(50) NOT NULL UNIQUE,
    email VARCHAR(100) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    first_name VARCHAR(50),
    last_name VARCHAR(50),
    phone_number VARCHAR(20),
    profile_image_url VARCHAR(255),
    date_of_birth DATE,
    is_verified BOOLEAN DEFAULT FALSE,
    verification_token VARCHAR(100),
    reset_password_token VARCHAR(100),
    reset_token_expires TIMESTAMP,
    last_login_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN DEFAULT TRUE
);
COMMENT ON TABLE public.users IS 'Stores user account information and authentication details';

CREATE TABLE public.roles (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE public.roles IS 'Defines different user roles in the system for permission management';

CREATE TABLE public.user_roles (
    user_id UUID REFERENCES public.users(id) ON DELETE CASCADE,
    role_id BIGINT REFERENCES public.roles(id) ON DELETE CASCADE,
    assigned_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, role_id)
);
COMMENT ON TABLE public.user_roles IS 'Maps users to their assigned roles for permission control';

CREATE TABLE public.permissions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE public.permissions IS 'Defines available permissions that can be assigned to roles';

CREATE TABLE public.role_permissions (
    role_id BIGINT REFERENCES public.roles(id) ON DELETE CASCADE,
    permission_id BIGINT REFERENCES public.permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);
COMMENT ON TABLE public.role_permissions IS 'Maps roles to their assigned permissions';

-- ========================
-- Event Management Section
-- ========================

CREATE TABLE public.event_categories (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    parent_category_id BIGINT REFERENCES public.event_categories(id),
    icon_url VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE public.event_categories IS 'Categorizes events by type with hierarchical structure';

CREATE TABLE public.venues (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name VARCHAR(200) NOT NULL,
    address TEXT NOT NULL,
    city VARCHAR(100) NOT NULL,
    state VARCHAR(100),
    country VARCHAR(100) NOT NULL,
    postal_code VARCHAR(20),
    latitude DECIMAL(10, 8),
    longitude DECIMAL(11, 8),
    capacity INT,
    contact_info TEXT,
    website VARCHAR(255),
    description TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE public.venues IS 'Stores locations where events can be held';

CREATE TABLE public.events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title VARCHAR(200) NOT NULL,
    description TEXT,
    short_description VARCHAR(500),
    category_id BIGINT REFERENCES public.event_categories(id),
    organizer_id UUID REFERENCES public.users(id),
    venue_id BIGINT REFERENCES public.venues(id),
    start_date TIMESTAMP NOT NULL,
    end_date TIMESTAMP NOT NULL,
    registration_start_date TIMESTAMP NOT NULL,
    registration_end_date TIMESTAMP NOT NULL,
    is_featured BOOLEAN DEFAULT FALSE,
    is_private BOOLEAN DEFAULT FALSE,
    status VARCHAR(20) DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'cancelled', 'completed')),
    max_attendees INT,
    thumbnail_url VARCHAR(255),
    banner_url VARCHAR(255),
    website VARCHAR(255),
    contact_email VARCHAR(100),
    contact_phone VARCHAR(20),
    terms_and_conditions TEXT,
    additional_info TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE public.events IS 'Core table storing all event information';

CREATE TABLE public.event_images (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id UUID REFERENCES public.events(id) ON DELETE CASCADE,
    image_url VARCHAR(255) NOT NULL,
    caption VARCHAR(200),
    display_order INT DEFAULT 0,
    is_primary BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE public.event_images IS 'Stores images associated with events';

CREATE TABLE public.tags (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE public.tags IS 'Contains tags that can be applied to events for categorization';

CREATE TABLE public.event_tags (
    event_id UUID REFERENCES public.events(id) ON DELETE CASCADE,
    tag_id BIGINT REFERENCES public.tags(id) ON DELETE CASCADE,
    PRIMARY KEY (event_id, tag_id)
);
COMMENT ON TABLE public.event_tags IS 'Maps events to their associated tags';

-- ========================
-- Ticket Management Section
-- ========================

CREATE TABLE public.ticket_types (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    benefits TEXT,
    color_code VARCHAR(10),
    icon_url VARCHAR(255),
    display_order INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE public.ticket_types IS 'Defines different types of tickets that can be created';

CREATE TABLE public.tickets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_id UUID REFERENCES public.events(id) ON DELETE CASCADE,
    ticket_type_id BIGINT REFERENCES public.ticket_types(id),
    name VARCHAR(100) NOT NULL,
    description TEXT,
    price DECIMAL(10, 2) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    max_tickets_per_order INT DEFAULT 10,
    is_transferable BOOLEAN DEFAULT TRUE,
    is_refundable BOOLEAN DEFAULT FALSE,
    refund_policy TEXT,
    sales_start_date TIMESTAMP NOT NULL,
    sales_end_date TIMESTAMP NOT NULL,
    status VARCHAR(20) DEFAULT 'active' CHECK (status IN ('active', 'sold_out', 'unavailable', 'hidden')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE public.tickets IS 'Defines ticketing options available for each event';

CREATE TABLE public.ticket_inventory (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ticket_id UUID REFERENCES public.tickets(id) ON DELETE CASCADE,
    total_quantity INT NOT NULL,
    available_quantity INT NOT NULL,
    reserved_quantity INT DEFAULT 0,
    sold_quantity INT DEFAULT 0,
    last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT check_quantities CHECK (total_quantity >= (available_quantity + reserved_quantity + sold_quantity))
);
COMMENT ON TABLE public.ticket_inventory IS 'Tracks availability of tickets for each event';

CREATE TABLE public.ticket_price_tiers (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ticket_id UUID REFERENCES public.tickets(id) ON DELETE CASCADE,
    tier_name VARCHAR(100) NOT NULL,
    price DECIMAL(10, 2) NOT NULL,
    start_date TIMESTAMP NOT NULL,
    end_date TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE public.ticket_price_tiers IS 'Supports time-based pricing for tickets';

CREATE TABLE public.ticket_reservations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ticket_id UUID REFERENCES public.tickets(id) ON DELETE CASCADE,
    user_id UUID REFERENCES public.users(id) ON DELETE CASCADE,
    quantity INT NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT check_quantity CHECK (quantity > 0)
);
COMMENT ON TABLE public.ticket_reservations IS 'Tracks ticket reservations for users';

-- ========================
-- Orders and Payments Section
-- ========================

CREATE TABLE public.discounts (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    code VARCHAR(50) NOT NULL UNIQUE,
    description TEXT,
    discount_type VARCHAR(20) NOT NULL CHECK (discount_type IN ('percentage', 'fixed_amount')),
    discount_value DECIMAL(10, 2) NOT NULL,
    min_purchase_amount DECIMAL(10, 2) DEFAULT 0,
    max_discount_amount DECIMAL(10, 2),
    start_date TIMESTAMP NOT NULL,
    end_date TIMESTAMP NOT NULL,
    usage_limit INT,
    usage_count INT DEFAULT 0,
    is_active BOOLEAN DEFAULT TRUE,
    created_by UUID REFERENCES public.users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE public.discounts IS 'Stores promotional codes and discounts';

CREATE TABLE public.event_discounts (
    discount_id BIGINT REFERENCES public.discounts(id) ON DELETE CASCADE,
    event_id UUID REFERENCES public.events(id) ON DELETE CASCADE,
    PRIMARY KEY (discount_id, event_id)
);
COMMENT ON TABLE public.event_discounts IS 'Maps discounts to specific events';

CREATE TABLE public.orders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    customer_id UUID REFERENCES public.users(id),
    order_number VARCHAR(50) NOT NULL UNIQUE,
    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    subtotal DECIMAL(10, 2) NOT NULL,
    discount_amount DECIMAL(10, 2) DEFAULT 0,
    tax_amount DECIMAL(10, 2) DEFAULT 0,
    fee_amount DECIMAL(10, 2) DEFAULT 0,
    total_amount DECIMAL(10, 2) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    discount_id BIGINT REFERENCES public.discounts(id),
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'paid', 'cancelled', 'refunded', 'failed')),
    billing_name VARCHAR(100),
    billing_email VARCHAR(100),
    billing_address TEXT,
    billing_city VARCHAR(100),
    billing_state VARCHAR(100),
    billing_country VARCHAR(100),
    billing_postal_code VARCHAR(20),
    billing_phone VARCHAR(20),
    notes TEXT,
    ip_address VARCHAR(50),
    user_agent TEXT,
    expiry_date TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE public.orders IS 'Stores customer orders with billing information';

CREATE TABLE public.order_items (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    order_id UUID REFERENCES public.orders(id) ON DELETE CASCADE,
    ticket_id UUID REFERENCES public.tickets(id),
    quantity INT NOT NULL CHECK (quantity > 0),
    unit_price DECIMAL(10, 2) NOT NULL,
    subtotal DECIMAL(10, 2) NOT NULL,
    discount_amount DECIMAL(10, 2) DEFAULT 0,
    tax_amount DECIMAL(10, 2) DEFAULT 0,
    fee_amount DECIMAL(10, 2) DEFAULT 0,
    total_amount DECIMAL(10, 2) NOT NULL,
    status VARCHAR(20) DEFAULT 'active' CHECK (status IN ('active', 'cancelled', 'refunded')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE public.order_items IS 'Individual line items within an order';

CREATE TABLE public.ticket_instances (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_item_id BIGINT REFERENCES public.order_items(id) ON DELETE CASCADE,
    ticket_id UUID REFERENCES public.tickets(id),
    ticket_code VARCHAR(100) UNIQUE,
    qr_code_url VARCHAR(255),
    barcode_url VARCHAR(255),
    attendee_name VARCHAR(100),
    attendee_email VARCHAR(100),
    check_in_status VARCHAR(20) DEFAULT 'not_checked_in' CHECK (check_in_status IN ('not_checked_in', 'checked_in', 'invalid')),
    check_in_time TIMESTAMP,
    is_transferred BOOLEAN DEFAULT FALSE,
    original_owner_id UUID REFERENCES public.users(id),
    current_owner_id UUID REFERENCES public.users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE public.ticket_instances IS 'Individual ticket instances issued to customers';

CREATE TABLE public.payments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id UUID REFERENCES public.orders(id) ON DELETE CASCADE,
    payment_method VARCHAR(50) NOT NULL,
    payment_provider VARCHAR(50) NOT NULL,
    transaction_id VARCHAR(100),
    amount DECIMAL(10, 2) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'completed', 'failed', 'refunded', 'partially_refunded')),
    payment_date TIMESTAMP,
    provider_response TEXT,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE public.payments IS 'Tracks payment transactions for orders';

CREATE TABLE public.refunds (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    payment_id UUID REFERENCES public.payments(id),
    order_id UUID REFERENCES public.orders(id),
    amount DECIMAL(10, 2) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    reason TEXT,
    refund_transaction_id VARCHAR(100),
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN ('pending', 'completed', 'failed')),
    refunded_at TIMESTAMP,
    created_by UUID REFERENCES public.users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE public.refunds IS 'Records refund transactions';

-- ========================
-- Additional Features Section
-- ========================

CREATE TABLE public.reviews (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id UUID REFERENCES public.events(id) ON DELETE CASCADE,
    user_id UUID REFERENCES public.users(id),
    rating INT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    title VARCHAR(200),
    comment TEXT,
    is_approved BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE public.reviews IS 'Stores user reviews and ratings for events';

CREATE TABLE public.notifications (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES public.users(id) ON DELETE CASCADE,
    title VARCHAR(200) NOT NULL,
    message TEXT NOT NULL,
    notification_type VARCHAR(50) NOT NULL,
    related_entity_type VARCHAR(50),
    related_entity_id VARCHAR(50),
    is_read BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE public.notifications IS 'System notifications for users';

CREATE TABLE public.event_attendees (
    event_id UUID REFERENCES public.events(id) ON DELETE CASCADE,
    user_id UUID REFERENCES public.users(id) ON DELETE CASCADE,
    registration_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    attendance_status VARCHAR(20) DEFAULT 'registered' CHECK (attendance_status IN ('registered', 'attended', 'no_show', 'cancelled')),
    check_in_time TIMESTAMP,
    check_out_time TIMESTAMP,
    notes TEXT,
    PRIMARY KEY (event_id, user_id)
);
COMMENT ON TABLE public.event_attendees IS 'Tracks attendance at events';

CREATE TABLE public.audit_logs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id UUID REFERENCES public.users(id),
    action VARCHAR(100) NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    entity_id VARCHAR(50),
    previous_state JSONB,
    current_state JSONB,
    ip_address VARCHAR(50),
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE public.audit_logs IS 'Security audit trail of system activities';

CREATE TABLE public.sessions (
    id VARCHAR(100) PRIMARY KEY,
    user_id UUID REFERENCES public.users(id) ON DELETE CASCADE,
    ip_address VARCHAR(50),
    user_agent TEXT,
    is_valid BOOLEAN DEFAULT TRUE,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_activity TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE public.sessions IS 'User session information for authentication';

-- ========================
-- Indexes for performance
-- ========================

-- User related indexes
CREATE INDEX idx_users_email ON public.users(email);
CREATE INDEX idx_users_username ON public.users(username);

-- Event related indexes
CREATE INDEX idx_events_start_date ON public.events(start_date);
CREATE INDEX idx_events_category ON public.events(category_id);
CREATE INDEX idx_events_organizer ON public.events(organizer_id);
CREATE INDEX idx_events_venue ON public.events(venue_id);
CREATE INDEX idx_events_status ON public.events(status);

-- Ticket related indexes
CREATE INDEX idx_tickets_event ON public.tickets(event_id);
CREATE INDEX idx_tickets_price ON public.tickets(price);
CREATE INDEX idx_ticket_inventory_ticket ON public.ticket_inventory(ticket_id);

-- Order related indexes
CREATE INDEX idx_orders_customer ON public.orders(customer_id);
CREATE INDEX idx_orders_status ON public.orders(status);
CREATE INDEX idx_orders_date ON public.orders(order_date);
CREATE INDEX idx_order_items_order ON public.order_items(order_id);
CREATE INDEX idx_order_items_ticket ON public.order_items(ticket_id);

-- Payment related indexes
CREATE INDEX idx_payments_order ON public.payments(order_id);
CREATE INDEX idx_payments_status ON public.payments(status);
CREATE INDEX idx_refunds_payment ON public.refunds(payment_id);

-- Notification indexes
CREATE INDEX idx_notifications_user ON public.notifications(user_id);
CREATE INDEX idx_notifications_read ON public.notifications(is_read);

-- ========================
-- Initial Data Insertion
-- ========================

-- Insert roles
INSERT INTO public.roles 
    (name, description) 
VALUES 
    ('admin', 'System administrator with full access'),
    ('organizer', 'Event organizer who can create and manage events'),
    ('customer', 'Regular user who can purchase tickets');

-- Insert permissions
INSERT INTO public.permissions 
    (name, description) 
VALUES
    ('manage_users', 'Create, update, delete users'),
    ('manage_events', 'Create, update, delete events'),
    ('manage_tickets', 'Create, update, delete tickets'),
    ('manage_venues', 'Create, update, delete venues'),
    ('manage_orders', 'View and manage orders'),
    ('process_payments', 'Process payments and refunds'),
    ('view_reports', 'Access to system reports and analytics'),
    ('manage_discounts', 'Create and manage discount codes');

-- Assign permissions to admin role
INSERT INTO public.role_permissions 
    (role_id, permission_id)
SELECT 
    r.id, p.id
FROM 
    public.roles r
CROSS JOIN 
    public.permissions p
WHERE 
    r.name = 'admin';

-- Assign permissions to organizer role
INSERT INTO public.role_permissions 
    (role_id, permission_id)
SELECT 
    r.id, p.id
FROM 
    public.roles r
JOIN 
    public.permissions p
ON 
    p.name IN (
        'manage_events', 
        'manage_tickets', 
        'manage_venues', 
        'manage_orders', 
        'view_reports', 
        'manage_discounts'
    )
WHERE 
    r.name = 'organizer';

-- Insert event categories
INSERT INTO public.event_categories 
    (name, description) 
VALUES
    ('Concert', 'Live music performances'),
    ('Conference', 'Professional gatherings and talks'),
    ('Workshop', 'Interactive learning sessions'),
    ('Festival', 'Celebrations and cultural events'),
    ('Sports', 'Athletic competitions and sporting events'),
    ('Exhibition', 'Art shows and exhibitions'),
    ('Theater', 'Theatrical performances and plays'),
    ('Networking', 'Professional networking events');

-- Insert ticket types
INSERT INTO public.ticket_types 
    (name, description, benefits, color_code) 
VALUES
    ('VIP', 'Premium experience with exclusive benefits', 'Priority access, exclusive areas, complimentary items', '#FFD700'),
    ('Standard', 'Regular admission to the event', 'General access to all public areas', '#1E90FF'),
    ('Early Bird', 'Discounted tickets for early purchasers', 'Same as standard but at a reduced price', '#32CD32'),
    ('Student', 'Discounted tickets for students', 'Same as standard but requires student ID', '#9370DB'),
    ('Group', 'Discounted tickets for groups', 'Bulk purchase discount', '#FF6347');

-- Create function for updating timestamps
CREATE OR REPLACE FUNCTION update_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply the trigger to all tables with updated_at column
DO $$
DECLARE
    t text;
BEGIN
    FOR t IN 
        SELECT 
            table_name 
        FROM 
            information_schema.columns 
        WHERE 
            column_name = 'updated_at' 
            AND table_schema = 'public'
    LOOP
        EXECUTE format('
            CREATE TRIGGER update_%I_timestamp
            BEFORE UPDATE ON public.%I
            FOR EACH ROW 
            EXECUTE FUNCTION update_timestamp()', 
            t, t);
    END LOOP;
END;
$$ LANGUAGE plpgsql;