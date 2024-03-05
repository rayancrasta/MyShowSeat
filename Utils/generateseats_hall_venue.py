## Chatgpt generated 
import psycopg2
from faker import Faker
import random
import string

# Connect to your PostgreSQL database
conn = psycopg2.connect(
    host="localhost",
    database="tickets",
    user="rayanc",
    password="rayanc"
)

# Create a cursor object to execute SQL queries
cursor = conn.cursor()

# Function to generate random data and insert into the Venue table
def generate_venue_data(num_records):
    fake = Faker()

    for _ in range(num_records):
        venue_name = fake.company()
        venue_location = fake.city()

        # Execute the SQL query to insert data into the Venue table
        cursor.execute("INSERT INTO Venue (VenueName, VenueLocation) VALUES (%s, %s) RETURNING VenueID", (venue_name, venue_location))

        # Fetch the VenueID of the newly inserted record
        venue_id = cursor.fetchone()[0]

        # Call the function to generate and insert random data for Hall and Seat tables
        generate_hall_and_seat_data(venue_id)

    # Commit the changes to the database
    conn.commit()

# Function to generate random data and insert into the Hall and Seat tables
def generate_hall_and_seat_data(venue_id):
    fake = Faker()

    # Generate data for the Hall table
    cursor.execute("INSERT INTO Hall (VenueID) VALUES (%s) RETURNING HallID", (venue_id,))
    hall_id = cursor.fetchone()[0]

    # Dictionary to store prices for each category within a hall
    category_prices = {}

    # Generate data for the Seat table
    for seat_number in range(1, 101):  # 100 seats for every hall
        seat_id = f"{venue_id}-{hall_id}-{seat_number}"
        category = random.choice(["VIP", "Regular", "Economy"])

        # Check if the category already has a price in the current hall
        if category in category_prices:
            price = category_prices[category]
        else:
            # If not, generate a new price and store it in the dictionary
            price = random.uniform(20.0, 100.0)
            category_prices[category] = price

        # Execute the SQL query to insert data into the Seat table
        cursor.execute("INSERT INTO Seat (SeatID, HallID, VenueID, Price, Category) VALUES (%s, %s, %s, %s, %s)",
                       (seat_id, hall_id, venue_id, price, category))

    # Commit the changes for the Seat table
    conn.commit()

# Specify the number of random records to generate for Venue
num_venue_records = 5

# Call the function to generate and insert random data for Venue, Hall, and Seat tables
generate_venue_data(num_venue_records)

# Close the cursor and connection
cursor.close()
conn.close()
