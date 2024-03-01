## Chatgpt generated 
import psycopg2
from faker import Faker
import random
from datetime import datetime, timedelta

# Connect to your PostgreSQL database
conn = psycopg2.connect(
    host="localhost",
    database="tickets",
    user="rayanc",
    password="rayanc"
)


# Create a cursor object to execute SQL queries
cursor = conn.cursor()

# Function to check if there are existing venues and halls
def check_existing_venues_and_halls():
    cursor.execute("SELECT COUNT(*) FROM Venue")
    num_venues = cursor.fetchone()[0]

    cursor.execute("SELECT COUNT(*) FROM Hall")
    num_halls = cursor.fetchone()[0]

    return num_venues > 0 and num_halls > 0

# Function to generate random data and insert into the Show table
def generate_show_data(num_records):
    if not check_existing_venues_and_halls():
        print("No existing venues and halls found. Please generate venues and halls first.")
        return

    fake = Faker()

    # Get a list of existing VenueIDs and HallIDs
    cursor.execute("SELECT Venue.VenueID, Hall.HallID FROM Venue, Hall WHERE Venue.VenueID = Hall.VenueID")
    venue_hall_ids = cursor.fetchall()

    for _ in range(num_records):
        show_name = fake.word()
        venue_id, hall_id = random.choice(venue_hall_ids)
        capacity = random.randint(50, 200)
        
        # Generate random start and end times for the show
        time_start = fake.date_time_between(start_date="-30d", end_date="+30d")
        time_end = time_start + timedelta(hours=random.randint(1, 5))

        # Execute the SQL query to insert data into the Show table
        cursor.execute("INSERT INTO Show (ShowName, VenueID, HallID, Capacity, Time_start, Time_end) VALUES (%s, %s, %s, %s, %s, %s)",
                       (show_name, venue_id, hall_id, capacity, time_start, time_end))

    # Commit the changes to the database
    conn.commit()

# Specify the number of random records to generate for Show table
num_show_records = 10

# Call the function to generate and insert random data for the Show table
generate_show_data(num_show_records)

# Close the cursor and connection
cursor.close()
conn.close()
